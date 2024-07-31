package cmdhandler

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

var extCommands = CommandMap{
	InstallKey:   cmd{f: install, Name: InstallName, ShouldReportStatus: false, pre: nil, failExitCode: 52},
	UninstallKey: cmd{f: uninstall, Name: UninstallName, ShouldReportStatus: false, pre: nil, failExitCode: 3},
	EnableKey:    cmd{f: enable, Name: EnableName, ShouldReportStatus: true, pre: enablePre, failExitCode: 3},
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
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("Starting AppHealth Extension %s seqNum=%d operation=%s", GetExtensionVersion(), seqNum, command.Name.String()))
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("HandlerEnviroment = %s", hEnv))
	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.KillVMWatchTask, "Received shutdown request")
		global.Shutdown = true
		err := vmwatch.KillVMWatch(lg, vmwatch.VMWatchCommand)
		if err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.KillVMWatchTask, fmt.Sprintf("Error when killing vmwatch process, error: %s", err.Error()))
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
	fullName = "Microsoft.ManagedServices.ApplicationHealthLinux"
	dataDir  = "/var/lib/waagent/apphealth" // TODO: This doesn't seem to be used anywhere since new Logger uses LogFolder
)

func install(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create data dir")
	}
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Created data dir", "path", dataDir)
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Handler successfully installed")
	return "", nil
}

func uninstall(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	// lg.Info(fmt.Sprintf("removing data dir, path: %s", dataDir), slog.String("path", dataDir))
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Removing data dir", "path", dataDir)
	if err := os.RemoveAll(dataDir); err != nil {
		return "", errors.Wrap(err, "failed to delete data dir")
	}
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Successfully removed data dir")
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.AppHealthTask, "Handler successfully uninstalled")
	return "", nil
}

func enablePre(lg *slog.Logger, seqNum uint) error {
	// exit if this sequence number (a snapshot of the configuration) is already
	// processed. if not, save this sequence number before proceeding.

	mrSeqNum, err := seqnoManager.GetCurrentSequenceNumber(lg, fullName, "")
	if err != nil {
		return errors.Wrap(err, "failed to get current sequence number")
	}
	// If the most recent sequence number is greater than or equal to the requested sequence number,
	// then the script has already been run and we should exit.
	if mrSeqNum != 0 && seqNum < mrSeqNum {
		lg.Info("the script configuration has already been processed, will not run again")
		return errors.Errorf("most recent sequence number %d is greater than the requested sequence number %d", mrSeqNum, seqNum)
	}

	// save the sequence number
	if err := seqnoManager.SetSequenceNumber(fullName, "", seqNum); err != nil {
		return errors.Wrap(err, "failed to save sequence number")
	}
	return nil
}
