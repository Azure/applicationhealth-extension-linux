package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/go-kit/log"
	"github.com/pkg/errors"
)

type cmdFunc func(lg log.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum int) (msg string, err error)
type preFunc func(lg log.Logger, seqNum int) error

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

func noop(lg log.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	lg.Log("event", "noop")
	return "", nil
}

func install(lg log.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}

	sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Created data dir", "path", dataDir)
	sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Handler successfully installed")
	return "", nil
}

func uninstall(lg log.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	{ // a new context scope with path
		lg = log.With(lg, "path", dataDir)
		sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Removing data dir")
		if err := os.RemoveAll(dataDir); err != nil {
			return "", errors.Wrap(err, "failed to delete data dir")
		}
		sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Successfully removed data dir")
	}
	sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Handler successfully uninstalled")
	return "", nil
}

const (
	statusMessage = "Successfully polling for application health"
)

var (
	errTerminated = errors.New("Application health process terminated")
)

func enable(lg log.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := parseAndValidateSettings(lg, h.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Successfully parsed and validated settings")
	sendTelemetry(lg, telemetry.EventLevelVerbose, telemetry.AppHealthTask, fmt.Sprintf("HandlerSettings = %s", cfg))

	probe := NewHealthProbe(lg, &cfg)
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
		sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Grace period not set")
	} else {
		sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
	}

	sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("VMWatch settings: %s", vmWatchSettings))
	if vmWatchSettings == nil || vmWatchSettings.Enabled == false {
		sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.StartVMWatchTask, "VMWatch is disabled, not starting process.")
	} else {
		vmWatchResult = VMWatchResult{Status: NotRunning, Error: nil}
		go executeVMWatch(lg, vmWatchSettings, h, vmWatchResultChannel)
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
		probeResponse, err := probe.evaluate(lg)
		state := probeResponse.ApplicationHealthState
		if err != nil {
			sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask,
				fmt.Sprintf("Error evaluating health probe: %v", err), "error", err)
		}

		if shutdown {
			sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Shutting down AppHealth Extension Gracefully")
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
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportHeatBeatTask, "VMWatch is running")
			} else if result.Status == Failed {
				sendTelemetry(lg, telemetry.EventLevelError, telemetry.ReportHeatBeatTask, vmWatchResult.GetMessage())
			} else if result.Status == NotRunning {
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportHeatBeatTask, "VMWatch is not running")
			}
		default:
			if vmWatchResult.Status == Running && time.Since(timeOfLastVMWatchLog) >= 60*time.Second {
				timeOfLastVMWatchLog = time.Now()
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportHeatBeatTask, "VMWatch is running")
			}
		}

		// Only increment if it's a repeat of the previous
		if prevState == state {
			numConsecutiveProbes++
			// Log stage changes and also reset consecutive count to 1 as a new state was observed
		} else {
			sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Health state changed to %s", strings.ToLower(string(state))))
			numConsecutiveProbes = 1
			prevState = state
		}

		if honorGracePeriod {
			timeElapsed := time.Now().Sub(gracePeriodStartTime)
			// If grace period expires, application didn't initialize on time
			if timeElapsed >= gracePeriodInSeconds {
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				state = probe.healthStatusAfterGracePeriodExpires()
				prevState = probe.healthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = Empty
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.healthStatusAfterGracePeriodExpires()) {
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == Empty) {
			if state != committedState {
				committedState = state
				sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
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

		err = reportStatusWithSubstatuses(lg, h, seqNum, StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			sendTelemetry(lg, telemetry.EventLevelError, telemetry.ReportStatusTask,
				fmt.Sprintf("Error while trying to report extension status with seqNum: %d, StatusType: %s, message: %s, substatuses: %#v, error: %s",
					seqNum,
					StatusSuccess,
					statusMessage,
					substatuses,
					err.Error()))
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
