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

	seqnoManager seqno.SequenceNumberManager = seqno.New()
)

func main() {
	if err != nil {
		slog.Error("failed to create logger", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}
	v, err := version.GetExtensionVersion()
	if err != nil {
		logger.Error("failed to get extension version", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}
	logger.With("version", v)

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

	seqNum, err := seqnum.FindSeqNum(hEnv.ConfigFolder) // find sequence number
	if err != nil {
		logger.Error("failed to find sequence number", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	// Initialize telemetry singleton, which can be used with package level function
	if _, err := telemetry.NewTelemetry(hEnv); err != nil {
		logger.Error(fmt.Sprintf("failed to initialize telemetry object, error: %s", err.Error()), slog.Any("error", err))
		os.Exit(cmd.failExitCode)
	}

	handler, err := cmdhandler.NewCommandHandler() // get the command handler
	if err != nil {
		logger.Error("failed to create command handler", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	logger, err = logging.NewExtensionLogger(hEnv) // create a new logger
	if err != nil {
		logger.Error("failed to create logger", slog.Any("error", err))
		exiter.Exit(exithelper.EnvironmentError)
	}

	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("Starting AppHealth Extension %s seqNum=%d operation=%s", GetExtensionVersion(), seqNum, cmd.name))
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("HandlerEnviroment = %s", hEnv))

	err = handler.Execute(cmdKey, hEnv, seqNum, logger) // execute the command
	if err != nil {
		logger.Error("failed to execute command", slog.Any("error", err))
		exiter.Exit(exithelper.ExecutionError)
	}
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("Finished execution of AppHealth Extension %s seqNum=%d operation=%s", GetExtensionVersion(), seqNum, cmd.name))
}
