package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

var (
	// dataDir is where we store the logs and state for the extension handler
	dataDir = "/var/lib/waagent/apphealth"

	shutdown = false

	// We need a reference to the command here so that we can cleanly shutdown VMWatch process
	// when a shutdown signal is received
	vmWatchCommand *exec.Cmd
)

func main() {
	logger := slog.New(logging.NewExtensionSlogHandler(os.Stdout, nil)).
		With("version", VersionString()).
		With("pid", os.Getpid())
	// parse command line arguments
	cmd := parseCmd(os.Args)
	logger = logger.With("operation", strings.ToLower(cmd.name))

	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.KillVMWatchTask, "Received shutdown request")
		shutdown = true
		err := killVMWatch(logger, vmWatchCommand)
		if err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.KillVMWatchTask, fmt.Sprintf("Error when killing vmwatch process, error: %s", err.Error()))
		}
	}()

	// parse extension environment
	hEnv, err := handlerenv.GetHandlerEnviroment()
	if err != nil {
		logger.Info("failed to parse handlerenv", "error", err)
		os.Exit(cmd.failExitCode)
	}
	seqNum, err := FindSeqNum(hEnv.ConfigFolder)
	if err != nil {
		logger.Info("failed to find sequence number", "error", err)
	}
	logger = logger.With("seq", seqNum)
	slog.SetDefault(logger)

	// Initialize telemetry singleton, which can be used with package level function
	if _, err := telemetry.NewTelemetry(hEnv); err != nil {
		logger.Error(fmt.Sprintf("failed to initialize telemetry object, error: %s", err.Error()), slog.Any("error", err))
		os.Exit(cmd.failExitCode)
	}
	// check sub-command preconditions, if any, before executing
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("Starting AppHealth Extension %s seqNum=%d operation=%s", GetExtensionVersion(), seqNum, cmd.name))
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("HandlerEnviroment = %s", hEnv))
	if cmd.pre != nil {
		logger.Info("pre-check")
		if err := cmd.pre(logger, seqNum); err != nil {
			telemetry.SendEvent(telemetry.ErrorEvent, telemetry.MainTask, "pre-check failed", "error", err.Error())
			os.Exit(cmd.failExitCode)
		}
	}
	// execute the subcommand
	reportStatus(logger, hEnv, seqNum, StatusTransitioning, cmd, "")
	msg, err := cmd.f(logger, hEnv, seqNum)
	if err != nil {
		logger.Error("failed to handle", "error", err)
		reportStatus(logger, hEnv, seqNum, StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	reportStatus(logger, hEnv, seqNum, StatusSuccess, cmd, msg)
	telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, fmt.Sprintf("Finished execution of AppHealth Extension %s seqNum=%d operation=%s", GetExtensionVersion(), seqNum, cmd.name))
}

// parseCmd looks at os.Args and parses the subcommand. If it is invalid,
// it prints the usage string and an error message and exits with code 0.
func parseCmd(args []string) cmd {
	if len(os.Args) != 2 {
		printUsage(args)
		fmt.Println("Incorrect usage.")
		os.Exit(2)
	}
	op := os.Args[1]
	cmd, ok := cmds[op]
	if !ok {
		printUsage(args)
		fmt.Printf("Incorrect command: %q\n", op)
		os.Exit(2)
	}
	return cmd
}

// printUsage prints the help string and version of the program to stdout with a
// trailing new line.
func printUsage(args []string) {
	fmt.Printf("Usage: %s ", os.Args[0])
	i := 0
	for k := range cmds {
		fmt.Printf(k)
		if i != len(cmds)-1 {
			fmt.Printf("|")
		}
		i++
	}
	fmt.Println()
	fmt.Println(DetailedVersionString())
}
