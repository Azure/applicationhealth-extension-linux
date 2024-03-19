package cmdhandler

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
	UpdateKey:    {noop, UpdateName, true, nil, 3},
	DisableKey:   {noop, DisableName, true, nil, 3},
}

type LinuxCommandHandler struct {
	commands CommandMap
}

func newOSCommandHandler() (CommandHandler, error) {
	return &LinuxCommandHandler{
		commands: extCommands,
	}, nil
}

var (
	// We need a reference to the command here so that we can cleanly shutdown VMWatch process
	// when a shutdown signal is received
	vmWatchCommand *exec.Cmd
)

func (ch *LinuxCommandHandler) CommandMap() CommandMap {
	return ch.commands
}

func (ch *LinuxCommandHandler) validateCommandToExecute(cmd CommandKey) error {
	if _, ok := ch.commands[cmd]; !ok {
		return errors.Errorf("unknown command: %s", cmd)
	}
	return nil
}

func (ch *LinuxCommandHandler) Execute(lg logging.Logger, c CommandKey, hEnv *handlerenv.HandlerEnvironment, seqNum int) error {
	err := ch.validateCommandToExecute(c) // set the command to execute
	if err != nil {
		return errors.Errorf("failed to find command to execute: %s", err)
	}

	// Getting command to execute
	cmd, ok := ch.commands[c]
	if !ok {
		return errors.Errorf("unknown command: %s", c)
	}

	lg.With("operation", strings.ToLower(cmd.Name.String()))
	lg.With("seq", strconv.Itoa(seqNum))

	lg.Info("Starting AppHealth Extension")
	lg.Info(fmt.Sprintf("HandlerEnvironment: %v", hEnv))
	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		lg.Info("Received shutdown request")
		global.Shutdown = true
		err := vmwatch.KillVMWatch(lg, vmWatchCommand)
		if err != nil {
			lg.Error("error when killing vmwatch", slog.Any("error", err))
		}
	}()

	if cmd.pre != nil {
		lg.Info("pre-check")
		if err := cmd.pre(lg, seqNum); err != nil {
			lg.Error("pre-check failed", slog.Any("error", err))
			os.Exit(cmd.failExitCode)
		}
	}

	// execute the subcommand
	ReportStatus(lg, hEnv, seqNum, status.StatusTransitioning, cmd, "")
	msg, err := cmd.f(lg, hEnv, seqNum)
	if err != nil {
		lg.Error("failed to handle", slog.Any("error", err))
		ReportStatus(lg, hEnv, seqNum, status.StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	ReportStatus(lg, hEnv, seqNum, status.StatusSuccess, cmd, msg)
	lg.Info("end")

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
	{ // a new context scope with path
		lg.With("path", dataDir)

		lg.Info(fmt.Sprintf("removing data dir, path: %s", dataDir))
		if err := os.RemoveAll(dataDir); err != nil {
			return "", errors.Wrap(err, "failed to delete data dir")
		}
		lg.Info("removed data dir")
	}
	lg.Info("uninstalled")
	return "", nil
}

func enable(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := settings.ParseAndValidateSettings(lg, h.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	probe := apphealth.NewHealthProbe(lg, &cfg.AppHealthPluginSettings)
	var (
		intervalBetweenProbesInMs = time.Duration(cfg.GetIntervalInSeconds()) * time.Millisecond * 1000
		numberOfProbes            = cfg.GetNumberOfProbes()
		gracePeriodInSeconds      = time.Duration(cfg.GetGracePeriod()) * time.Second
		numConsecutiveProbes      = 0
		prevState                 = apphealth.Empty
		committedState            = apphealth.Empty
		honorGracePeriod          = gracePeriodInSeconds > 0
		gracePeriodStartTime      = time.Now()
		vmWatchSettings           = cfg.GetVMWatchSettings()
		vmWatchResult             = vmwatch.VMWatchResult{Status: vmwatch.Disabled, Error: nil}
		vmWatchResultChannel      = make(chan vmwatch.VMWatchResult)
		timeOfLastVMWatchLog      = time.Time{}
	)

	if !honorGracePeriod {
		lg.Info("Grace period not set")
	} else {
		lg.Info(fmt.Sprintf("Grace period set to %v", gracePeriodInSeconds))
	}

	lg.Info(fmt.Sprintf("VMWatch settings: %#v", vmWatchSettings))
	if vmWatchSettings == nil || vmWatchSettings.Enabled == false {
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
		if err != nil {
			lg.Error("Error occurred during probe evaluation", slog.Any("error", err))
		}

		if global.Shutdown {
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
				lg.Info("VMWatch is running")
			} else if result.Status == vmwatch.Failed {
				lg.Error("VMWatch failed", slog.String("error", vmWatchResult.GetMessage()))
			}
		default:
			if vmWatchResult.Status == vmwatch.Running && time.Since(timeOfLastVMWatchLog) >= 60*time.Second {
				timeOfLastVMWatchLog = time.Now()
				lg.Info("VMWatch is running")
			}
		}

		// Only increment if it's a repeat of the previous
		if prevState == state {
			numConsecutiveProbes++
			// Log stage changes and also reset consecutive count to 1 as a new state was observed
		} else {
			lg.Info("Health state changed to " + strings.ToLower(string(state)))
			numConsecutiveProbes = 1
			prevState = state
		}

		if honorGracePeriod {
			timeElapsed := time.Now().Sub(gracePeriodStartTime)
			// If grace period expires, application didn't initialize on time
			if timeElapsed >= gracePeriodInSeconds {
				lg.Info(fmt.Sprintf("No longer honoring grace period - expired. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				state = probe.HealthStatusAfterGracePeriodExpires()
				prevState = probe.HealthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = apphealth.Empty
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.HealthStatusAfterGracePeriodExpires()) {
				lg.Info(fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				lg.Info(fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = apphealth.Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == apphealth.Empty) {
			if state != committedState {
				committedState = state
				lg.Info(fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
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

		if probeResponse.CustomMetrics != "" {
			customMetricsStatusType := status.StatusError
			if probeResponse.ValidateCustomMetrics() == nil {
				customMetricsStatusType = status.StatusSuccess
			}
			substatuses = append(substatuses, status.NewSubstatus(apphealth.SubstatusKeyNameCustomMetrics, customMetricsStatusType, probeResponse.CustomMetrics))
		}

		// VMWatch substatus should only be displayed when settings are present
		if vmWatchSettings != nil {
			substatuses = append(substatuses, status.NewSubstatus(vmwatch.SubstatusKeyNameVMWatch, vmWatchResult.Status.GetStatusType(), vmWatchResult.GetMessage()))
		}

		err = ReportStatusWithSubstatuses(lg, h, seqNum, status.StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			lg.Error("Failed to report status", slog.Any("error", err))
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
