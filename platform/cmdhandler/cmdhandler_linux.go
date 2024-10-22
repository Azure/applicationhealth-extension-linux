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
	"github.com/Azure/applicationhealth-extension-linux/internal/seqno"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

var extCommands = CommandMap{
	InstallKey:   cmd{f: install, Name: InstallName, ShouldReportStatus: false, pre: nil, failExitCode: 52},
	UninstallKey: cmd{f: uninstall, Name: UninstallName, ShouldReportStatus: false, pre: nil, failExitCode: 3},
	EnableKey:    cmd{f: enable, Name: EnableName, ShouldReportStatus: true, pre: enableHandler, failExitCode: 3},
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

func (ch *LinuxCommandHandler) Execute(c CommandKey, h *handlerenv.HandlerEnvironment, seqNum uint, lg *slog.Logger) error {
	// Getting command to execute
	v, err := version.GetExtensionVersion()
	if err != nil {
		lg.Error("failed to get extension version", slog.Any("error", err))
	}

	command, ok := ch.commands[c]
	if !ok {
		return errors.Errorf("unknown command: %s", c)
	}

	lg, err = logging.NewSlogLogger(h, "ApplicationHealth.log")
	if err != nil {
		return errors.Wrap(err, "failed to create logger")
	}
	lg = lg.With(
		"version", v,
		"pid", os.Getpid(),
		"operation", strings.ToLower(command.Name.String()),
		"seq", strconv.Itoa(int(seqNum)),
	)
	slog.SetDefault(lg)

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, "Starting AppHealth Extension")
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("HandlerEnvironment: %v", h))
	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, "Received shutdown request")
		global.Shutdown = true
		err := vmwatch.KillVMWatch(lg, vmwatch.VMWatchCommand)
		if err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.KillVMWatchTask, fmt.Sprintf("Error when killing vmwatch process, error: %s", err.Error()))
		}
	}()

	if command.pre != nil {
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, "Running Pre-check")
		if err := command.pre(lg, seqNum); err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.MainTask, fmt.Sprintf("Pre-check failed: %v", err))
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
