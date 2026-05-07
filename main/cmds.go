package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/pkg/errors"
)

type cmdFunc func(lg *slog.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum uint) (msg string, err error)
type preFunc func(lg *slog.Logger, seqNum uint) error

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
	cmdEnable    = cmd{enable, "Enable", true, enablePre, 3}
	cmdUninstall = cmd{uninstall, "Uninstall", false, nil, 3}

	cmds = map[string]cmd{
		"install":   cmdInstall,
		"uninstall": cmdUninstall,
		"enable":    cmdEnable,
		"update":    {noop, "Update", true, nil, 3},
		"disable":   {noop, "Disable", true, nil, 3},
	}
)

func noop(lg *slog.Logger, h *handlerenv.HandlerEnvironment, seqNum uint) (string, error) {
	lg.Info("noop")
	return "", nil
}

func install(lg *slog.Logger, h *handlerenv.HandlerEnvironment, seqNum uint) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Created data dir", "path", dataDir)
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Handler successfully installed")
	return "", nil
}

func uninstall(lg *slog.Logger, h *handlerenv.HandlerEnvironment, seqNum uint) (string, error) {
	{ // a new context scope with path
		slog.SetDefault(lg.With("path", dataDir))
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Removing data dir", "path", dataDir)
		if err := os.RemoveAll(dataDir); err != nil {
			return "", errors.Wrap(err, "failed to delete data dir")
		}
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Successfully removed data dir")
	}
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Handler successfully uninstalled")
	return "", nil
}

const (
	statusMessage = "Successfully polling for application health"
)

var (
	errTerminated        = errors.New("Application health process terminated")
	errIdempotentExit    = errors.New("idempotent exit: healthy process already running with current configuration")
	errSuperseded        = errors.New("process superseded by newer sequence number")
)

func enablePre(lg *slog.Logger, seqNum uint) error {
	// exit if this sequence number (a snapshot of the configuration) is already
	// processed. if not, save this sequence number before proceeding.

	mrSeqNum, err := seqnoManager.GetCurrentSequenceNumber(lg, fullName, "")
	if err != nil {
		return errors.Wrap(err, "failed to get current sequence number")
	}

	// Discover existing AHE processes once and reuse the result for both
	// idempotency checks and new-sequence-number cleanup.
	var existingPids []int
	pids, err := findExistingProcesses()
	if err != nil {
		telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
			fmt.Sprintf("Failed to discover existing processes: %v", err), "error", err)
	} else {
		existingPids = pids
		if len(pids) > 1 {
			telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Found %d existing AHE processes (PIDs: %v). Expected at most 1.", len(pids), pids),
				"pids", pids)
		}
	}

	// Check idempotency: if an existing healthy process is already running
	// with the same sequence number, exit cleanly. The log file mtime used
	// for freshness is captured by the shim before any output is redirected,
	// so it is not affected by logging from this process.
	if shouldExit := checkIdempotency(lg, seqNum, mrSeqNum, existingPids); shouldExit {
		return errIdempotentExit
	}

	// If the most recent sequence number is greater than or equal to the requested sequence number,
	// then the script has already been run and we should exit.
	if mrSeqNum != 0 && seqNum < mrSeqNum {
		lg.Info("the script configuration has already been processed, will not run again")
		return errors.Errorf("most recent sequence number %d is greater than the requested sequence number %d", mrSeqNum, seqNum)
	}

	// Save the sequence number before killing old processes, so that if a crash
	// occurs after killing, the sequence number is already recorded. This allows
	// the old process to detect the newer sequence in its loop and exit gracefully,
	// and ensures that any fresh app start will have the correct sequence state.
	if err := seqnoManager.SetSequenceNumber(fullName, "", seqNum); err != nil {
		return errors.Wrap(err, "failed to save sequence number")
	}

	// New sequence number — kill any existing processes from previous sequence
	if seqNum > mrSeqNum && len(existingPids) > 0 {
		lg.Info("Killing existing processes from previous sequence number", "pids", existingPids, "oldSeq", mrSeqNum, "newSeq", seqNum)
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
			fmt.Sprintf("Killing existing processes PIDs=%v from previous sequence number %d, new sequence number %d", existingPids, mrSeqNum, seqNum))
		killProcesses(existingPids)
	}

	return nil
}

// checkIdempotency checks if another AHE instance is already running and decides
// whether the current process should continue or exit based on sequence number
// and existing process health status.
// Returns true if the current process should exit (healthy process already running).
func checkIdempotency(lg *slog.Logger, seqNum uint, mrSeqNum uint, existingPids []int) bool {
	// Only apply idempotency logic for same sequence number
	if seqNum != mrSeqNum {
		return false
	}

	// Check if any other AHE process is running — do this before log freshness
	// to avoid unnecessary I/O and misleading telemetry when nothing is running.
	if len(existingPids) == 0 {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
			fmt.Sprintf("IsHandlerStillExecuting: Process Name='%s', result=False", AppHealthBinaryNameAmd64))
		return false
	}

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
		fmt.Sprintf("IsHandlerStillExecuting: Process Name='%s', PIDs=%v, result=True", AppHealthBinaryNameAmd64, existingPids),
		"pids", existingPids)

	// Read the handler log file's last write time, captured by the shim BEFORE
	// redirecting output to handler.log. This ensures we check the previous
	// process's last write time, not the current shim invocation's writes.
	lastUpdate, logFileErr := getLogFileLastWriteTimeFromEnv()

	// Could not determine log file timestamp after retries (e.g., file does not exist, I/O errors).
	// Treat as not responsive — if no log file exists, no previous process was writing heartbeats.
	if logFileErr != nil {
		telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
			fmt.Sprintf("Could not determine log file last write time: %v. Existing process appears unresponsive. PID %d taking over execution.",
				logFileErr, os.Getpid()),
			"error", logFileErr, "seqNum", seqNum, "existingPids", existingPids, "currentPid", os.Getpid())

		killProcesses(existingPids)
		return false
	}

	threshold := time.Duration(AppHealthLogFileStaleThresholdInMinutes) * time.Minute
	logFresh := time.Since(lastUpdate) < threshold

	if logFresh {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
			fmt.Sprintf("Existing process log file was fresh at startup. Last update: %s UTC.", lastUpdate.UTC().Format(time.RFC3339Nano)),
			"lastUpdate", lastUpdate.UTC().Format(time.RFC3339Nano))
	} else {
		telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
			fmt.Sprintf("Existing process log file was stale at startup. Last update: %s UTC, Threshold: %d minutes. Process may be stuck.",
				lastUpdate.UTC().Format(time.RFC3339Nano), AppHealthLogFileStaleThresholdInMinutes),
			"lastUpdate", lastUpdate.UTC().Format(time.RFC3339Nano),
			"thresholdMinutes", AppHealthLogFileStaleThresholdInMinutes)
	}

	// Same sequence number + process running + log fresh → caller should exit to maintain idempotency
	if logFresh {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
			fmt.Sprintf("Another instance of AppHealthExtension is already running with the same sequence number (%d) and is responsive. PID %d exiting to maintain idempotency.",
				seqNum, os.Getpid()),
			"seqNum", seqNum, "existingPids", existingPids, "currentPid", os.Getpid())
		return true
	}

	// Same sequence number + process running + log stale → take over execution
	telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
		fmt.Sprintf("Another instance of AppHealthExtension exists with the same sequence number (%d) but appears unresponsive (log file stale). PID %d taking over execution.",
			seqNum, os.Getpid()),
		"seqNum", seqNum, "existingPids", existingPids, "currentPid", os.Getpid())

	killProcesses(existingPids)

	return false
}

func enable(lg *slog.Logger, h *handlerenv.HandlerEnvironment, seqNum uint) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := parseAndValidateSettings(lg, h.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Successfully parsed and validated settings")
	telemetry.SendEvent(telemetry.VerboseEvent, telemetry.AppHealthTask, fmt.Sprintf("HandlerSettings = %s", cfg))

	probe := NewHealthProbe(lg, &cfg)
	var (
		intervalBetweenProbesInMs  = time.Duration(cfg.intervalInSeconds()) * time.Millisecond * 1000
		numberOfProbes             = cfg.numberOfProbes()
		gracePeriodInSeconds       = time.Duration(cfg.gracePeriod()) * time.Second
		numConsecutiveProbes       = 0
		prevState                  = HealthStatus(Empty)
		committedState             = HealthStatus(Empty)
		commitedCustomMetricsState = CustomMetricsStatus(Empty)
		honorGracePeriod           = gracePeriodInSeconds > 0
		gracePeriodStartTime       = time.Now()
		vmWatchSettings            = cfg.vmWatchSettings()
		vmWatchResult              = VMWatchResult{Status: Disabled, Error: nil}
		vmWatchResultChannel       = make(chan VMWatchResult)
		timeOfLastVMWatchLog       = time.Time{}
	)

	if !honorGracePeriod {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Grace period not set")
	} else {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
	}

	// Try to set VMWatchCohortId as extension event operationId
	if vmWatchSettings != nil {
		if vmWatchCohortId, err := vmWatchSettings.TryGetVMWatchCohortId(); err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.SetupVMWatchTask, "Error while getting VMWatchCohortId", "error", err)
		} else if vmWatchCohortId != "" {
			telemetry.SetOperationID(vmWatchCohortId)
		}
	}

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("VMWatch settings: %s", vmWatchSettings))
	if vmWatchSettings == nil || vmWatchSettings.Enabled == false {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.StartVMWatchTask, "VMWatch is disabled, not starting process.")
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
		// Check if a newer sequence number has been started by another process.
		// If so, this process has a stale configuration and should exit gracefully.
		mostRecentSequenceNumberStarted, err := seqnoManager.GetCurrentSequenceNumber(lg, fullName, "")
		if err != nil {
			telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Failed to read current sequence number: %v. Continuing with current configuration.", err), "error", err)
		} else if seqNum < mostRecentSequenceNumberStarted {
			telemetry.SendEvent(telemetry.WarningEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Current sequence number %d is not greater than the most recently started sequence number %d. PID %d initiating graceful shutdown.",
					seqNum, mostRecentSequenceNumberStarted, os.Getpid()),
				"sequenceNumber", seqNum, "mostRecentSequenceNumberStarted", mostRecentSequenceNumberStarted, "currentPid", os.Getpid())
			return "", errSuperseded
		}

		// Since we only log health state changes, it is possible there will be no recent logs for app health extension.
		// As an indication that the extension is running, we log app health extension heart beat at a set interval.
		LogHeartBeat()

		startTime := time.Now()
		probeResponse, err := probe.evaluate(lg)
		state := probeResponse.ApplicationHealthState
		customMetrics := probeResponse.CustomMetrics
		if err != nil {
			telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask,
				fmt.Sprintf("Error evaluating health probe: %v", err), "error", err)
		}

		if shutdown {
			telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Shutting down AppHealth Extension Gracefully")
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
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.ReportHeatBeatTask, "VMWatch is running")
			} else if result.Status == Failed {
				telemetry.SendEvent(telemetry.ErrorEvent, telemetry.ReportHeatBeatTask, vmWatchResult.GetMessage())
			} else if result.Status == NotRunning {
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.ReportHeatBeatTask, "VMWatch is not running")
			}
		default:
			if vmWatchResult.Status == Running && time.Since(timeOfLastVMWatchLog) >= 60*time.Second {
				timeOfLastVMWatchLog = time.Now()
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.ReportHeatBeatTask, "VMWatch is running")
			}
		}

		// Only increment if it's a repeat of the previous
		if prevState == state {
			numConsecutiveProbes++
			// Log stage changes and also reset consecutive count to 1 as a new state was observed
		} else {
			telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("Health state changed to %s", strings.ToLower(string(state))))
			numConsecutiveProbes = 1
			prevState = state
		}

		if honorGracePeriod {
			timeElapsed := time.Now().Sub(gracePeriodStartTime)
			// If grace period expires, application didn't initialize on time
			if timeElapsed >= gracePeriodInSeconds {
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				state = probe.healthStatusAfterGracePeriodExpires()
				prevState = probe.healthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = HealthStatus(Empty)
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.healthStatusAfterGracePeriodExpires()) {
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == HealthStatus(Empty)) {
			if state != committedState {
				committedState = state
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
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

		if customMetrics != Empty {
			customMetricsStatusType := StatusError
			if probeResponse.validateCustomMetrics() == nil {
				customMetricsStatusType = StatusSuccess
			}
			substatuses = append(substatuses, NewSubstatus(SubstatusKeyNameCustomMetrics, customMetricsStatusType, customMetrics))
			if commitedCustomMetricsState != CustomMetricsStatus(customMetrics) {
				telemetry.SendEvent(telemetry.InfoEvent, telemetry.ReportStatusTask,
					fmt.Sprintf("Reporting CustomMetric Substatus with status: %s , message: %s", customMetricsStatusType, customMetrics))
				commitedCustomMetricsState = CustomMetricsStatus(customMetrics)
			}
		}

		// VMWatch substatus should only be displayed when settings are present
		if vmWatchSettings != nil {
			substatuses = append(substatuses, NewSubstatus(SubstatusKeyNameVMWatch, vmWatchResult.Status.GetStatusType(), vmWatchResult.GetMessage()))
		}

		err = reportStatusWithSubstatuses(lg, h, seqNum, StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.ReportStatusTask,
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
