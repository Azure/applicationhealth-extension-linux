package main

import (
	"log/slog"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/seqnum"
	"github.com/Azure/applicationhealth-extension-linux/platform/cmdhandler"
	"github.com/Azure/azure-extension-platform/pkg/exithelper"
)

var (
	// the logger that will be used throughout
	lg, err = logging.NewExtensionLogger(nil)
	// Exit helper
	eh = exithelper.Exiter
)

func main() {
	if err != nil {
		slog.Error("failed to create logger", slog.Any("error", err))
		eh.Exit(exithelper.EnvironmentError)
	}
	lg.With("version", version.VersionString())

	cmdKey, err := cmdhandler.ParseCmd() // parse command line arguments
	if err != nil {
		lg.Error("failed to parse command", slog.Any("error", err))
		eh.Exit(exithelper.ArgumentError)
	}

	hEnv, err := handlerenv.GetHandlerEnviroment() // parse handler environment
	if err != nil {
		lg.Error("failed to parse Handler Enviroment", slog.Any("error", err))
		eh.Exit(exithelper.EnvironmentError)
	}

	seqNum, err := seqnum.FindSeqNum(hEnv.ConfigFolder) // find sequence number
	if err != nil {
		lg.Error("failed to find sequence number", slog.Any("error", err))
		eh.Exit(exithelper.EnvironmentError)
	}

	handler, err := cmdhandler.NewCommandHandler() // get the command handler
	if err != nil {
		lg.Error("failed to create command handler", slog.Any("error", err))
		eh.Exit(exithelper.EnvironmentError)
	}

	err = handler.Execute(lg, cmdKey, hEnv, seqNum) // execute the command
	if err != nil {
		lg.Error("failed to execute command", slog.Any("error", err))
		eh.Exit(exithelper.ExecutionError)
	}
	lg.Info("end")
}
