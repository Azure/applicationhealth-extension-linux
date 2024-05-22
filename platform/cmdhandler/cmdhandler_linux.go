package cmdhandler

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/platform/settings"
	"github.com/Azure/applicationhealth-extension-linux/plugins/apphealth"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

var extCommands = CommandMap{
	InstallKey:   cmd{f: install, Name: InstallName, ShouldReportStatus: false, pre: nil, failExitCode: 52},
	UninstallKey: cmd{f: uninstall, Name: UninstallName, ShouldReportStatus: false, pre: nil, failExitCode: 3},
	EnableKey:    cmd{f: enable, Name: EnableName, ShouldReportStatus: true, pre: nil, failExitCode: 3},
	UpdateKey:    cmd{f: noop, Name: UpdateName, ShouldReportStatus: true, pre: nil, failExitCode: 3},
	DisableKey:   cmd{f: noop, Name: DisableName, ShouldReportStatus: true, pre: nil, failExitCode: 3},
}

type LinuxCommandHandler struct {
	commands CommandMap
}

func newOSCommandHandler() (CommandHandler, error) {
	return &LinuxCommandHandler{
		commands: extCommands,
	}, nil
}

func (ch *LinuxCommandHandler) CommandMap() CommandMap {
	return ch.commands
}

func (ch *LinuxCommandHandler) Execute(c CommandKey, h *handlerenv.HandlerEnvironment, seqNum int, lg logging.Logger) error {
	// Getting command to execute
	command, ok := ch.commands[c]
	if !ok {
		return errors.Errorf("unknown command: %s", c)
	}

	lg.With("operation", strings.ToLower(command.Name.String()))
	lg.With("seq", strconv.Itoa(seqNum))

	lg.Info("Starting AppHealth Extension")
	lg.Info(fmt.Sprintf("HandlerEnvironment: %v", h))
	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		lg.Info("Received shutdown request")
		global.Shutdown = true
		err := vmwatch.KillVMWatch(lg, vmwatch.VMWatchCommand)
		if err != nil {
			lg.Error("error when killing vmwatch", slog.Any("error", err))
		}
	}()

	if command.pre != nil {
		lg.Info("pre-check")
		if err := command.pre(lg, seqNum); err != nil {
			lg.Error("pre-check failed", slog.Any("error", err))
			os.Exit(command.failExitCode)
		}
	}

	// execute the subcommand
	ReportStatus(lg, h, seqNum, status.StatusTransitioning, command, "")
	msg, err := command.f(lg, h, seqNum)
	if err != nil {
		lg.Error("failed to handle", slog.Any("error", err))
		ReportStatus(lg, h, seqNum, status.StatusError, command, err.Error()+msg)
		os.Exit(command.failExitCode)
	}
	ReportStatus(lg, h, seqNum, status.StatusSuccess, command, msg)
	return nil
}

const (
	fullName      = "Microsoft.ManagedServices.ApplicationHealthLinux"
	statusMessage = "Successfully polling for application health"
	dataDir       = "/var/lib/waagent/apphealth" // TODO: This doesn't seem to be used anywhere since new Logger uses LogFolder
)

var (
	errTerminated = errors.New("Application health process terminated")
)

func install(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}

	lg.Info(fmt.Sprintf("created data dir, path: %s", dataDir))
	lg.Info("installed")
	return "", nil
}

func uninstall(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	lg.Info(fmt.Sprintf("removing data dir, path: %s", dataDir), slog.String("path", dataDir))
	if err := os.RemoveAll(dataDir); err != nil {
		return "", errors.Wrap(err, "failed to delete data dir")
	}
	lg.Info("removed data dir", slog.String("path", dataDir))
	lg.Info("uninstalled", slog.String("path", dataDir))
	return "", nil
}

func enable(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := settings.ParseAndValidateSettings(lg, h.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Successfully parsed and validated settings")
	// sendTelemetry(lg, telemetry.EventLevelVerbose, telemetry.AppHealthTask, fmt.Sprintf("HandlerSettings = %s", cfg))

	probe := apphealth.NewHealthProbe(lg, &cfg.AppHealthPluginSettings)
	var (
		intervalBetweenProbesInMs  = time.Duration(cfg.GetIntervalInSeconds()) * time.Millisecond * 1000
		numberOfProbes             = cfg.GetNumberOfProbes()
		gracePeriodInSeconds       = time.Duration(cfg.GetGracePeriod()) * time.Second
		numConsecutiveProbes       = 0
		prevState                  = apphealth.HealthStatus(apphealth.Empty)
		committedState             = apphealth.HealthStatus(apphealth.Empty)
		commitedCustomMetricsState = apphealth.CustomMetricsStatus(apphealth.Empty)
		honorGracePeriod           = gracePeriodInSeconds > 0
		gracePeriodStartTime       = time.Now()
		vmWatchSettings            = cfg.GetVMWatchSettings()
		vmWatchResult              = vmwatch.VMWatchResult{Status: vmwatch.Disabled, Error: nil}
		vmWatchResultChannel       = make(chan vmwatch.VMWatchResult)
		timeOfLastVMWatchLog       = time.Time{}
	)

	if !honorGracePeriod {
		// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Grace period not set")
		lg.Info("Grace period not set")
	} else {
		// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
		lg.Info(fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
	}

	lg.Info(fmt.Sprintf("VMWatch settings: %#v", vmWatchSettings))
	// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("VMWatch settings: %s", vmWatchSettings))
	if vmWatchSettings == nil || vmWatchSettings.Enabled == false {
		// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.StartVMWatchTask, "VMWatch is disabled, not starting process.")
		lg.Info("VMWatch is disabled, not starting process.")
	} else {
		vmWatchResult = vmwatch.VMWatchResult{Status: vmwatch.NotRunning, Error: nil}
		go vmwatch.ExecuteVMWatch(lg, vmWatchSettings, h, vmWatchResultChannel)
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
		probeResponse, err := probe.Evaluate(lg)
		state := probeResponse.ApplicationHealthState
		customMetrics := probeResponse.CustomMetrics
		if err != nil {
			lg.Error("Error occurred during probe evaluation", slog.Any("error", err))
			// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask,
			// 	fmt.Sprintf("Error evaluating health probe: %v", err), "error", err)
		}

		if global.Shutdown {
			// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Shutting down AppHealth Extension Gracefully")
			return "", errTerminated
		}

		// If VMWatch was never supposed to run, it will be in Disabled state, so we do not need to read from the channel
		// If VMWatch failed to execute, we will do not need to read from the channel
		// Only if VMWatch is currently running do we need to check if it failed
		select {
		case result, ok := <-vmWatchResultChannel:
			vmWatchResult = result
			if !ok {
				vmWatchResult = vmwatch.VMWatchResult{Status: vmwatch.Failed, Error: errors.New("VMWatch channel has closed, unknown error")}
			} else if result.Status == vmwatch.Running {
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportHeatBeatTask, "VMWatch is running")
				lg.Info("VMWatch is running")
			} else if result.Status == vmwatch.Failed {
				lg.Error("VMWatch failed", slog.String("error", vmWatchResult.GetMessage()))
				// sendTelemetry(lg, telemetry.EventLevelError, telemetry.ReportHeatBeatTask, vmWatchResult.GetMessage())
			} else if result.Status == vmwatch.NotRunning {
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportHeatBeatTask, "VMWatch is not running")
			}
		default:
			if vmWatchResult.Status == vmwatch.Running && time.Since(timeOfLastVMWatchLog) >= 60*time.Second {
				timeOfLastVMWatchLog = time.Now()
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportHeatBeatTask, "VMWatch is running")
				lg.Info("VMWatch is running")
			}
		}

		// Only increment if it's a repeat of the previous
		if prevState == state {
			numConsecutiveProbes++
			// Log stage changes and also reset consecutive count to 1 as a new state was observed
		} else {
			lg.Info("Health state changed to " + strings.ToLower(string(state)))
			// sendTselemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Health state changed to %s", strings.ToLower(string(state))))
			numConsecutiveProbes = 1
			prevState = state
		}

		if honorGracePeriod {
			timeElapsed := time.Now().Sub(gracePeriodStartTime)
			// If grace period expires, application didn't initialize on time
			if timeElapsed >= gracePeriodInSeconds {
				lg.Info(fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				state = probe.HealthStatusAfterGracePeriodExpires()
				prevState = probe.HealthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = apphealth.HealthStatus(apphealth.Empty)
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.HealthStatusAfterGracePeriodExpires()) {
				lg.Info(fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				lg.Info(fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = apphealth.Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == apphealth.HealthStatus(apphealth.Empty)) {
			if state != committedState {
				committedState = state
				lg.Info(fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
			}
			// Only reset if we've observed consecutive probes in order to preserve previous observations when handling grace period
			if numConsecutiveProbes == numberOfProbes {
				numConsecutiveProbes = 0
			}
		}

		substatuses := []status.SubstatusItem{
			// For V2 of extension, to remain backwards compatible with HostGAPlugin and to have HealthStore signals
			// decided by extension instead of taking a change in HostGAPlugin, first substatus will be dedicated
			// for health store.
			status.NewSubstatus(apphealth.SubstatusKeyNameAppHealthStatus, committedState.GetStatusTypeForAppHealthStatus(), committedState.GetMessageForAppHealthStatus()),
			status.NewSubstatus(apphealth.SubstatusKeyNameApplicationHealthState, committedState.GetStatusType(), string(committedState)),
		}

		if customMetrics != string(apphealth.Empty) {
			customMetricsStatusType := status.StatusError
			if probeResponse.ValidateCustomMetrics() == nil {
				customMetricsStatusType = status.StatusSuccess
			}
			substatuses = append(substatuses, status.NewSubstatus(apphealth.SubstatusKeyNameCustomMetrics, customMetricsStatusType, customMetrics))
			if commitedCustomMetricsState != apphealth.CustomMetricsStatus(customMetrics) {
				// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportStatusTask,
				// 	fmt.Sprintf("Reporting CustomMetric Substatus with status: %s , message: %s", customMetricsStatusType, customMetrics))
				commitedCustomMetricsState = apphealth.CustomMetricsStatus(customMetrics)
			}
		}

		// VMWatch substatus should only be displayed when settings are present
		if vmWatchSettings != nil {
			substatuses = append(substatuses, status.NewSubstatus(vmwatch.SubstatusKeyNameVMWatch, vmWatchResult.Status.GetStatusType(), vmWatchResult.GetMessage()))
		}

		err = ReportStatusWithSubstatuses(lg, h, seqNum, status.StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			lg.Error("Failed to report status", slog.Any("error", err))
			// sendTelemetry(lg, telemetry.EventLevelError, telemetry.ReportStatusTask,
			// 	fmt.Sprintf("Error while trying to report extension status with seqNum: %d, StatusType: %s, message: %s, substatuses: %#v, error: %s",
			// 		seqNum,
			// 		status.StatusSuccess,
			// 		statusMessage,
			// 		substatuses,
			// 		err.Error()))
		}

		endTime := time.Now()
		durationToWait := intervalBetweenProbesInMs - endTime.Sub(startTime)
		if durationToWait > 0 {
			time.Sleep(durationToWait)
		}

		if global.Shutdown {
			return "", errTerminated
		}
	}
}
