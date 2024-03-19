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
	logger, err = logging.NewExtensionLogger(nil)
	// Exit helper
	exiter = exithelper.Exiter
)

func main() {
	if err != nil {
		slog.Error("failed to create logger", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}
	logger.With("version", version.VersionString())

	cmdKey, err := cmdhandler.ParseCmd() // parse command line arguments
	if err != nil {
		logger.Error("failed to parse command", slog.Any("error", err))
		exiter.Exit(exithelper.ArgumentError)
	}

	hEnv, err := handlerenv.GetHandlerEnviroment() // parse handler environment
	if err != nil {
		logger.Error("failed to parse Handler Enviroment", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	seqNum, err := seqnum.FindSeqNum(hEnv.ConfigFolder) // find sequence number
	if err != nil {
		logger.Error("failed to find sequence number", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	handler, err := cmdhandler.NewCommandHandler() // get the command handler
	if err != nil {
		logger.Error("failed to create command handler", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	err = handler.Execute(cmdKey, hEnv, seqNum, logger) // execute the command
	if err != nil {
		logger.Error("failed to execute command", slog.Any("error", err))
		exiter.Exit(exithelper.ExecutionError)
	}
	logger.Info("end")
}
