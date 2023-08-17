package main

import (
	"fmt"
	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"os"
	"strings"
	"time"
)

type cmdFunc func(ctx *log.Context, hEnv vmextension.HandlerEnvironment, seqNum int) (msg string, err error)
type preFunc func(ctx *log.Context, seqNum int) error

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

func noop(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	ctx.Log("event", "noop")
	return "", nil
}

func install(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}

	ctx.Log("event", "created data dir", "path", dataDir)
	ctx.Log("event", "installed")
	return "", nil
}

func uninstall(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
	{ // a new context scope with path
		ctx = ctx.With("path", dataDir)
		ctx.Log("event", "removing data dir", "path", dataDir)
		if err := os.RemoveAll(dataDir); err != nil {
			return "", errors.Wrap(err, "failed to delete data dir")
		}
		ctx.Log("event", "removed data dir")
	}
	ctx.Log("event", "uninstalled")
	return "", nil
}

const (
	statusMessage = "Successfully polling for application health"
)

var (
	errTerminated = errors.New("Application health process terminated")
)

func enable(ctx *log.Context, h vmextension.HandlerEnvironment, seqNum int) (string, error) {
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
		vmWatchSettings	      	  = cfg.vmWatchSettings()
		vmWatchResult			  = VMWatchResult {Status: Disabled, Error: nil}
		vmWatchResultChannel	  = make(chan VMWatchResult)
	)

	if !honorGracePeriod {
		ctx.Log("event", "Grace period not set")
	} else {
		ctx.Log("event", fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
	}

	if (vmWatchSettings.Enabled) {
		ctx.Log("event", fmt.Sprintf("Attempting to run VMWatch with settings: %#v", vmWatchSettings))

		vmWatchCommand, err := vmWatchSettings.ToExecutableCommand()
		if (err != nil) {
			ctx.Log("error", err)
			vmWatchResult = VMWatchResult{Status: Failed, Error: err}
		} else {
			go executeVMWatch(ctx, vmWatchCommand, vmWatchResultChannel)
		}
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
			ctx.Log("error", err)
		}

		if shutdown {
			return "", errTerminated
		}

		// Only increment if it's a repeat of the previous
		if prevState == state {
			numConsecutiveProbes++
			// Log stage changes and also reset consecutive count to 1 as a new state was observed
		} else {
			ctx.Log("event", "Health state changed to "+strings.ToLower(string(state)))
			numConsecutiveProbes = 1
			prevState = state
		}

		if honorGracePeriod {
			timeElapsed := time.Now().Sub(gracePeriodStartTime)
			// If grace period expires, application didn't initialize on time
			if timeElapsed >= gracePeriodInSeconds {
				ctx.Log("event", fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				state = probe.healthStatusAfterGracePeriodExpires()
				prevState = probe.healthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = Empty
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.healthStatusAfterGracePeriodExpires()) {
				ctx.Log("event", fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				ctx.Log("event", fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == Empty) {
			if state != committedState {
				committedState = state
				ctx.Log("event", fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
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

		// VMWatch will be built-in to GuestHealthFramework, so we will always display VMWatch as substatus
		substatuses = append(substatuses, NewSubstatus(SubstatusKeyNameVMWatch, vmWatchResult.Status.GetStatusType(), vmWatchResult.GetMessage()))

		err = reportStatusWithSubstatuses(ctx, h, seqNum, StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			ctx.Log("error", err)
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
