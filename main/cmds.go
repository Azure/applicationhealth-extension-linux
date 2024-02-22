package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/pkg/errors"
)

type cmdFunc func(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (msg string, err error)
type preFunc func(ctx logging.ExtensionLogger, seqNum int) error

type cmd struct {
	f                  cmdFunc // associated function
	name               string  // human readable string
	shouldReportStatus bool    // determines if running this should log to a .status file
	pre                preFunc // executed before any status is reported
	failExitCode       int     // exitCode to use when commands fail
}

const (
	fullName = "Microsoft.ManagedServices.ApplicationHealthLinux"
)

var (
	cmdInstall   = cmd{install, "Install", false, nil, 52}
	cmdEnable    = cmd{enable, "Enable", true, nil, 3}
	cmdUninstall = cmd{uninstall, "Uninstall", false, nil, 3}

	cmds = map[string]cmd{
		"install":   cmdInstall,
		"uninstall": cmdUninstall,
		"enable":    cmdEnable,
		"update":    {noop, "Update", true, nil, 3},
		"disable":   {noop, "Disable", true, nil, 3},
	}
)

func noop(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	ctx.Event("noop")
	return "", nil
}

func install(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}

	ctx.Event(fmt.Sprintf("created data dir, path: %s", dataDir))
	ctx.Event("installed")
	return "", nil
}

func uninstall(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	{ // a new context scope with path
		ctx.With("path", dataDir)

		ctx.Event(fmt.Sprintf("removing data dir, path: %s", dataDir))
		if err := os.RemoveAll(dataDir); err != nil {
			return "", errors.Wrap(err, "failed to delete data dir")
		}
		ctx.Event("removed data dir")
	}
	ctx.Event("uninstalled")
	return "", nil
}

const (
	statusMessage = "Successfully polling for application health"
)

var (
	errTerminated = errors.New("Application health process terminated")
)

func enable(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := parseAndValidateSettings(ctx, h.HandlerEnvironment.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	probe := NewHealthProbe(ctx, &cfg)
	var (
		intervalBetweenProbesInMs = time.Duration(cfg.intervalInSeconds()) * time.Millisecond * 1000
		numberOfProbes            = cfg.numberOfProbes()
		gracePeriodInSeconds      = time.Duration(cfg.gracePeriod()) * time.Second
		numConsecutiveProbes      = 0
		prevState                 = Empty
		committedState            = Empty
		honorGracePeriod          = gracePeriodInSeconds > 0
		gracePeriodStartTime      = time.Now()
		vmWatchSettings           = cfg.vmWatchSettings()
		vmWatchResult             = VMWatchResult{Status: Disabled, Error: nil}
		vmWatchResultChannel      = make(chan VMWatchResult)
		timeOfLastVMWatchLog      = time.Time{}
	)

	if !honorGracePeriod {
		ctx.Event("Grace period not set")

	} else {
		ctx.Event(fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
	}

	ctx.Event(fmt.Sprintf("VMWatch settings: %#v", vmWatchSettings))
	if vmWatchSettings == nil || vmWatchSettings.Enabled == false {
		ctx.Event("VMWatch is disabled, not starting process.")
	} else {
		vmWatchResult = VMWatchResult{Status: NotRunning, Error: nil}
		go executeVMWatch(ctx, vmWatchSettings, h, vmWatchResultChannel)
	}

	// The committed health status (the state written to the status file) initially does not have a state
	// In order to change the state in the status file, the following must be observed:
	//  1. Healthy status observed once when committed state is unknown
	//  2. A different status is observed numberOfProbes consecutive times
	// Example: Committed state = healthy, numberOfProbes = 3
	// In order to change committed state to unhealthy, the probe needs to be unhealthy 3 consecutive times
	//
	// The committed health state will remain in 'Initializing' state until any of the following occurs:
	//	1. Grace period expires, then application will either be Unknown/Unhealthy depending on probe type
	//	2. A valid health state is observed numberOfProbes consecutive times
	for {
		startTime := time.Now()
		probeResponse, err := probe.evaluate(ctx)
		state := probeResponse.ApplicationHealthState
		if err != nil {
			ctx.EventError("Error occurred during probe evaluation", err)
		}

		if shutdown {
			return "", errTerminated
		}

		// If VMWatch was never supposed to run, it will be in Disabled state, so we do not need to read from the channel
		// If VMWatch failed to execute, we will do not need to read from the channel
		// Only if VMWatch is currently running do we need to check if it failed
		select {
		case result, ok := <-vmWatchResultChannel:
			vmWatchResult = result
			if !ok {
				vmWatchResult = VMWatchResult{Status: Failed, Error: errors.New("VMWatch channel has closed, unknown error")}
			} else if result.Status == Running {
				ctx.Event("VMWatch is running")
			} else if result.Status == Failed {
				ctx.EventError("VMWatch failed", vmWatchResult.GetMessage())
			}
		default:
			if vmWatchResult.Status == Running && time.Since(timeOfLastVMWatchLog) >= 60*time.Second {
				timeOfLastVMWatchLog = time.Now()
				ctx.Event("VMWatch is running")
			}
		}

		// Only increment if it's a repeat of the previous
		if prevState == state {
			numConsecutiveProbes++
			// Log stage changes and also reset consecutive count to 1 as a new state was observed
		} else {
			ctx.Event("Health state changed to " + strings.ToLower(string(state)))
			numConsecutiveProbes = 1
			prevState = state
		}

		if honorGracePeriod {
			timeElapsed := time.Now().Sub(gracePeriodStartTime)
			// If grace period expires, application didn't initialize on time
			if timeElapsed >= gracePeriodInSeconds {
				ctx.Event(fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				state = probe.healthStatusAfterGracePeriodExpires()
				prevState = probe.healthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = Empty
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.healthStatusAfterGracePeriodExpires()) {
				ctx.Event(fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				ctx.Event(fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == Empty) {
			if state != committedState {
				committedState = state
				ctx.Event(fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
			}
			// Only reset if we've observed consecutive probes in order to preserve previous observations when handling grace period
			if numConsecutiveProbes == numberOfProbes {
				numConsecutiveProbes = 0
			}
		}

		substatuses := []SubstatusItem{
			// For V2 of extension, to remain backwards compatible with HostGAPlugin and to have HealthStore signals
			// decided by extension instead of taking a change in HostGAPlugin, first substatus will be dedicated
			// for health store.
			NewSubstatus(SubstatusKeyNameAppHealthStatus, committedState.GetStatusTypeForAppHealthStatus(), committedState.GetMessageForAppHealthStatus()),
			NewSubstatus(SubstatusKeyNameApplicationHealthState, committedState.GetStatusType(), string(committedState)),
		}

		if probeResponse.CustomMetrics != "" {
			customMetricsStatusType := StatusError
			if probeResponse.validateCustomMetrics() == nil {
				customMetricsStatusType = StatusSuccess
			}
			substatuses = append(substatuses, NewSubstatus(SubstatusKeyNameCustomMetrics, customMetricsStatusType, probeResponse.CustomMetrics))
		}

		// VMWatch substatus should only be displayed when settings are present
		if vmWatchSettings != nil {
			substatuses = append(substatuses, NewSubstatus(SubstatusKeyNameVMWatch, vmWatchResult.Status.GetStatusType(), vmWatchResult.GetMessage()))
		}

		err = reportStatusWithSubstatuses(ctx, h, seqNum, StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			ctx.EventError("Failed to report status", err)
		}

		endTime := time.Now()
		durationToWait := intervalBetweenProbesInMs - endTime.Sub(startTime)
		if durationToWait > 0 {
			time.Sleep(durationToWait)
		}

		if shutdown {
			return "", errTerminated
		}
	}
}
