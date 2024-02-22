package cmdhandler

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/plugins/apphealth"
	"github.com/Azure/applicationhealth-extension-linux/plugins/settings"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

var extCommands = CommandMap{
	Install:   Cmd{f: install, Name: InstallName, ShouldReportStatus: false, pre: nil, failExitCode: 52},
	Uninstall: Cmd{f: uninstall, Name: UninstallName, ShouldReportStatus: false, pre: nil, failExitCode: 3},
	Enable:    Cmd{f: enable, Name: EnableName, ShouldReportStatus: true, pre: nil, failExitCode: 3},
	Update:    {noop, UpdateName, true, nil, 3},
	Disable:   {noop, DisableName, true, nil, 3},
}

type LinuxCommandHandler struct {
	commands CommandMap
	target   CommandKey
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

func (h *LinuxCommandHandler) CommandMap() CommandMap {
	return h.commands
}

func (h *LinuxCommandHandler) SetCommandToExecute(cmd CommandKey) error {
	if _, ok := h.commands[cmd]; !ok {
		return errors.Errorf("unknown command: %s", cmd)
	}
	h.target = cmd
	return nil
}

func (ch *LinuxCommandHandler) Execute(hEnv handlerenv.HandlerEnvironment, seqNum int) error {
	lg := logging.New(&hEnv)
	lg.WithProcessID()
	lg.WithTimestamp()
	lg.WithVersion(version.VersionString())

	if ch.target == "" {
		return errors.New("no command to execute")
	}

	// Getting command to execute
	cmd, ok := ch.commands[ch.target]
	if !ok {
		return errors.Errorf("unknown command: %s", ch.target)
	}

	lg.WithOperation(strings.ToLower(cmd.Name.String()))
	lg.WithSeqNum(strconv.Itoa(seqNum))

	lg.Event("Starting AppHealth Extension")
	lg.Event(fmt.Sprintf("HandlerEnvironment: %v", hEnv))
	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		lg.Event("Received shutdown request")
		global.Shutdown = true
		err := vmwatch.KillVMWatch(*lg, vmWatchCommand)
		if err != nil {
			lg.EventError("error when killing vmwatch", err)
		}
	}()

	if cmd.pre != nil {
		lg.Event("pre-check")
		if err := cmd.pre(*lg, seqNum); err != nil {
			lg.EventError("pre-check failed", err)
			os.Exit(cmd.failExitCode)
		}
	}

	// execute the subcommand
	ReportStatus(*lg, hEnv, seqNum, status.StatusTransitioning, cmd, "")
	msg, err := cmd.f(*lg, hEnv, seqNum)
	if err != nil {
		lg.EventError("failed to handle", err)
		ReportStatus(*lg, hEnv, seqNum, status.StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	ReportStatus(*lg, hEnv, seqNum, status.StatusSuccess, cmd, msg)
	lg.Event("end")

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

// func enable(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
// 	return "", nil
// }

func enable(ctx logging.ExtensionLogger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	// parse the extension handler settings (not available prior to 'enable')
	cfg, err := settings.ParseAndValidateSettings(ctx, h.HandlerEnvironment.ConfigFolder)
	if err != nil {
		return "", errors.Wrap(err, "failed to get configuration")
	}

	appHealthSettings := apphealth.NewAppHealthSettings(&cfg)

	probe := apphealth.NewHealthProbe(ctx, &cfg)
	var (
		intervalBetweenProbesInMs = time.Duration(appHealthSettings.GetIntervalInSeconds()) * time.Millisecond * 1000
		numberOfProbes            = appHealthSettings.GetNumberOfProbes()
		gracePeriodInSeconds      = time.Duration(appHealthSettings.GetGracePeriod()) * time.Second
		numConsecutiveProbes      = 0
		prevState                 = apphealth.Empty
		committedState            = apphealth.Empty
		honorGracePeriod          = gracePeriodInSeconds > 0
		gracePeriodStartTime      = time.Now()
		vmWatchSettings           = cfg.VMWatchSettings()
		vmWatchResult             = vmwatch.VMWatchResult{Status: vmwatch.Disabled, Error: nil}
		vmWatchResultChannel      = make(chan vmwatch.VMWatchResult)
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
		vmWatchResult = vmwatch.VMWatchResult{Status: vmwatch.NotRunning, Error: nil}
		go vmwatch.ExecuteVMWatch(ctx, vmWatchSettings, h, vmWatchResultChannel)
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
		probeResponse, err := probe.Evaluate(ctx)
		state := probeResponse.ApplicationHealthState
		if err != nil {
			ctx.EventError("Error occurred during probe evaluation", err)
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
				ctx.Event("VMWatch is running")
			} else if result.Status == vmwatch.Failed {
				ctx.EventError("VMWatch failed", vmWatchResult.GetMessage())
			}
		default:
			if vmWatchResult.Status == vmwatch.Running && time.Since(timeOfLastVMWatchLog) >= 60*time.Second {
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
				state = probe.HealthStatusAfterGracePeriodExpires()
				prevState = probe.HealthStatusAfterGracePeriodExpires()
				numConsecutiveProbes = 1
				committedState = apphealth.Empty
				// If grace period has not expired, check if we have consecutive valid probes
			} else if (numConsecutiveProbes == numberOfProbes) && (state != probe.HealthStatusAfterGracePeriodExpires()) {
				ctx.Event(fmt.Sprintf("No longer honoring grace period - successful probes. Time elapsed = %v", timeElapsed))
				honorGracePeriod = false
				// Application will be in Initializing state since we have not received consecutive valid health states
			} else {
				ctx.Event(fmt.Sprintf("Honoring grace period. Time elapsed = %v", timeElapsed))
				state = apphealth.Initializing
			}
		}

		if (numConsecutiveProbes == numberOfProbes) || (committedState == apphealth.Empty) {
			if state != committedState {
				committedState = state
				ctx.Event(fmt.Sprintf("Committed health state is %s", strings.ToLower(string(committedState))))
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

		err = ReportStatusWithSubstatuses(ctx, h, seqNum, status.StatusSuccess, "enable", statusMessage, substatuses)
		if err != nil {
			ctx.EventError("Failed to report status", err)
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
