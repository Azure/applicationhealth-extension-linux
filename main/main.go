package main

import (
	"log/slog"

	"github.com/Azure/applicationhealth-extension-linux/internal/cmdhandler"
	"github.com/Azure/applicationhealth-extension-linux/internal/exithelper"
	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/seqnum"
)

var (
	// the logger that will be used throughout
	lg = logging.NewExtensionLogger(nil)
	// Exit helper
	eh = exithelper.Exiter
)

func main() {
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

	seqNum, err := seqnum.FindSeqNum(hEnv.HandlerEnvironment.ConfigFolder) // find sequence number
	if err != nil {
		lg.Error("failed to find sequence number", slog.Any("error", err))
		eh.Exit(exithelper.HandlerError)
	}

	handler, err := cmdhandler.NewCommandHandler() // get the command handler
	if err != nil {
		lg.Error("failed to create command handler", slog.Any("error", err))
		eh.Exit(exithelper.HandlerError)
	}

	err = handler.SetCommandToExecute(cmdKey) // set the command to execute
	if err != nil {
		lg.Error("failed to find command to execute", slog.Any("error", err))
		eh.Exit(exithelper.HandlerError)
	}

	err = handler.Execute(hEnv, seqNum) // execute the command
	if err != nil {
		lg.Error("failed to execute command", slog.Any("error", err))
	}
	lg.Info("end")
}
