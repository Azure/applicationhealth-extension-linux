package cmdhandler

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/platform/settings"
	"github.com/Azure/applicationhealth-extension-linux/plugins/apphealth"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

type CommandKey string
type CommandName string

func (c CommandName) String() string {
	return string(c)
}

func (c CommandKey) String() string {
	return string(c)
}

const (
	InstallKey   CommandKey = "install"
	UninstallKey CommandKey = "uninstall"
	EnableKey    CommandKey = "enable"
	UpdateKey    CommandKey = "update"
	DisableKey   CommandKey = "disable"
)

const (
	InstallName   CommandName = "Install"
	UninstallName CommandName = "Uninstall"
	EnableName    CommandName = "Enable"
	UpdateName    CommandName = "Update"
	DisableName   CommandName = "Disable"
)

type cmdFunc func(lg *slog.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum uint) (msg string, err error)
type preFunc func(lg *slog.Logger, seqNum uint) error

type cmd struct {
	f                  cmdFunc     // associated function
	Name               CommandName // human readable string
	ShouldReportStatus bool        // determines if running this should log to a .status file
	pre                preFunc     // executed before any status is reported
	failExitCode       int         // exitCode to use when commands fail
}

type CommandMap map[CommandKey]cmd

// Get CommandMap Keys as list
func (cm CommandMap) Keys() []CommandKey {
	keys := make([]CommandKey, 0, len(cm))
	for k := range cm {
		keys = append(keys, k)
	}
	return keys
}

// Get CommandMap Values as list
func (cm CommandMap) Values() []cmd {
	values := make([]cmd, 0, len(cm))
	for _, v := range cm {
		values = append(values, v)
	}
	return values
}

type CommandHandler interface {
	Execute(c CommandKey, h *handlerenv.HandlerEnvironment, seqNum uint, lg *slog.Logger) error
	CommandMap() CommandMap
}

// returns a new CommandHandler depending on the OS
func NewCommandHandler() (CommandHandler, error) {
	handler, err := newOSCommandHandler()
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func noop(lg *slog.Logger, h *handlerenv.HandlerEnvironment, seqNum uint) (string, error) {
	lg.Info("noop")
	return "", nil
}

// parseCmd looks at os.Args and parses the subcommand. If it is invalid, it
// prints the usage string and returns an error
func ParseCmd() (CommandKey, error) {
	if len(os.Args) != 2 {
		printUsage(os.Args)
		return "", fmt.Errorf("Incorrect usage")
	}
	op := os.Args[1]
	// Check if the command is valid key defined in CommandKeys
	switch op {
	case InstallKey.String(), UninstallKey.String(), EnableKey.String(), UpdateKey.String(), DisableKey.String():
		return CommandKey(op), nil
	default:
		printUsage(os.Args)
		return "", fmt.Errorf("Incorrect command: %q\n", op)
	}
}

// printUsage prints the help string and version of the program to stdout with a
// trailing new line.
func printUsage(args []string) {
	fmt.Printf("Usage: %s ", os.Args[0])
	i := 0
	for k := range extCommands {
		if i > 0 {
			fmt.Print("|")
		}
		fmt.Print(k.String())
		i++
	}
	fmt.Println()
	fmt.Println(version.DetailedVersionString())
}

var (
	errTerminated = errors.New("Application health process terminated")
	statusMessage = "Successfully polling for application health"
)

func enable(lg *slog.Logger, h *handlerenv.HandlerEnvironment, seqNum uint) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := settings.ParseAndValidateSettings(lg, h.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	// sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.AppHealthTask, "Successfully parsed and validated settings")
	// sendTelemetry(lg, telemetry.EventLevelVerbose, telemetry.AppHealthTask, fmt.Sprintf("HandlerSettings = %s", cfg))

	probe := apphealth.NewHealthProbe(lg, &cfg.AppHealthPluginSettings)
	var (
		intervalBetweenProbes      = time.Duration(cfg.GetIntervalInSeconds()) * time.Millisecond * 1000 // seconds represented in milliseconds
		numberOfProbes             = cfg.GetNumberOfProbes()
		gracePeriod                = time.Duration(cfg.GetGracePeriod()) * time.Second // seconds
		numConsecutiveProbes       = 0
		prevState                  = apphealth.HealthStatus(apphealth.Empty)
		committedState             = apphealth.HealthStatus(apphealth.Empty)
		commitedCustomMetricsState = apphealth.CustomMetricsStatus(apphealth.Empty)
		honorGracePeriod           = gracePeriod > 0
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
		lg.Info(fmt.Sprintf("Grace period set to %v", gracePeriod))
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
				lg.Info("VMWatch is not running")
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
			if timeElapsed >= gracePeriod {
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
		durationToWait := intervalBetweenProbes - endTime.Sub(startTime)
		if durationToWait > 0 {
			time.Sleep(durationToWait)
		}

		if global.Shutdown {
			return "", errTerminated
		}
	}
}
