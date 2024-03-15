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

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	global "github.com/Azure/applicationhealth-extension-linux/internal/utils"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/Azure/applicationhealth-extension-linux/plugins/vmwatch"
	"github.com/pkg/errors"
)

var extCommands = CommandMap{
	InstallKey:   cmd{f: noop, Name: InstallName, ShouldReportStatus: false, pre: nil, failExitCode: 52},  // TODO: Implement
	UninstallKey: cmd{f: noop, Name: UninstallName, ShouldReportStatus: false, pre: nil, failExitCode: 3}, // TODO: Implement
	EnableKey:    cmd{f: enable, Name: EnableName, ShouldReportStatus: true, pre: nil, failExitCode: 3},
	UpdateKey:    {noop, UpdateName, true, nil, 3},
	DisableKey:   {noop, DisableName, true, nil, 3},
}

type WindowsCommandHandler struct {
	commands CommandMap
}

func newOSCommandHandler() (CommandHandler, error) {
	return &WindowsCommandHandler{
		commands: extCommands,
	}, nil
}

var (
	// We need a reference to the command here so that we can cleanly shutdown VMWatch process
	// when a shutdown signal is received
	vmWatchCommand *exec.Cmd
)

func (ch *WindowsCommandHandler) Execute(c CommandKey, h *handlerenv.HandlerEnvironment, seqNum int, lg logging.Logger) error {
	// TODO: Implement command execution
	lg.Info(fmt.Sprintf("WindowsCommandHandler was Created, with command: %s and sequenceNumber: %d", c, seqNum))

	// Getting command to execute
	cmd, ok := ch.commands[c]
	if !ok {
		return errors.Errorf("unknown command: %s", c)
	}

	lg.With("operation", strings.ToLower(cmd.Name.String()))
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
	ReportStatus(lg, h, seqNum, status.StatusTransitioning, cmd, "")
	msg, err := cmd.f(lg, h, seqNum)
	if err != nil {
		lg.Error("failed to handle", slog.Any("error", err))
		ReportStatus(lg, h, seqNum, status.StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	ReportStatus(lg, h, seqNum, status.StatusSuccess, cmd, msg)
	lg.Info("end")

	return nil
}

func (ch *WindowsCommandHandler) CommandMap() CommandMap {
	return ch.commands
}
