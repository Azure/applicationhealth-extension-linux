package main

import (
	"fmt"
	"log/slog"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/seqno"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/platform/cmdhandler"
	"github.com/Azure/azure-extension-platform/pkg/exithelper"
)

var (
	// the logger that will be used throughout
	logger, err = logging.NewSlogLogger(nil, "")
	// Exit helper
	exiter = exithelper.Exiter

	seqnoManager seqno.SequenceNumberManager = seqno.New()
)

func main() {
	if err != nil {
		slog.Error("failed to create logger", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}
	if err != nil {
		logger.Error("failed to get extension version", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	hEnv, err := handlerenv.GetHandlerEnviroment() // parse handler environment
	if err != nil {
		logger.Error("failed to parse Handler Enviroment", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	cmdKey, err := cmdhandler.ParseCmd() // parse command line arguments
	if err != nil {
		logger.Error("failed to parse command", slog.Any("error", err))
		exiter.Exit(exithelper.ArgumentError)
	}
	seqNum, err := seqnoManager.FindSeqNum(hEnv.ConfigFolder)
	if err != nil {
		logger.Info("failed to find sequence number", "error", err)
		exiter.Exit(exithelper.EnvironmentError)
	}
	// Initialize telemetry singleton, which can be used with package level function
	if _, err := telemetry.NewTelemetry(hEnv); err != nil {
		logger.Error(fmt.Sprintf("failed to initialize telemetry object, error: %s", err.Error()), slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	handler, err := cmdhandler.NewCommandHandler() // get the command handler
	if err != nil {
		logger.Error("failed to create command handler", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	if err != nil {
		logger.Error("failed to create logger", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	err = handler.Execute(cmdKey, hEnv, seqNum, logger) // execute the command
	if err != nil {
		logger.Error("failed to execute command", slog.Any("error", err))
		exiter.Exit(exithelper.ExecutionError)
	}
	logger.Info("end")
}
