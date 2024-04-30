package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/azure-extension-platform/pkg/extensionevents"
	"github.com/go-kit/log"
)

var (
	// dataDir is where we store the logs and state for the extension handler
	dataDir = "/var/lib/waagent/apphealth"

	shutdown = false

	// We need a reference to the command here so that we can cleanly shutdown VMWatch process
	// when a shutdown signal is received
	vmWatchCommand *exec.Cmd

	eem *extensionevents.ExtensionEventManager

	sendTelemetry telemetry.LogEventFunc
)

func main() {
	logger := log.NewSyncLogger(log.NewLogfmtLogger(
		os.Stdout))

	logger = log.With(logger, "time", log.DefaultTimestamp)
	logger = log.With(logger, "version", VersionString())

	// parse command line arguments
	cmd := parseCmd(os.Args)
	logger = log.With(logger, "operation", strings.ToLower(cmd.name))

	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		sendTelemetry(logger, telemetry.EventLevelInfo, telemetry.KillVMWatchTask, "Received shutdown request")
		shutdown = true
		err := killVMWatch(logger, vmWatchCommand)
		if err != nil {
			sendTelemetry(logger, telemetry.EventLevelError, telemetry.KillVMWatchTask, fmt.Sprintf("Error when killing vmwatch process, error: %s", err.Error()))
		}
	}()

	// parse extension environment
	hEnv, err := handlerenv.GetHandlerEnviroment()
	if err != nil {
		logger.Log("message", "failed to parse handlerenv", "error", err)
		os.Exit(cmd.failExitCode)
	}
	seqNum, err := FindSeqNum(hEnv.ConfigFolder)
	if err != nil {
		logger.Log("message", "failed to find sequence number", "error", err)
	}
	logger = log.With(logger, "seq", seqNum)
	eem = extensionevents.New(logging.NewNopLogger(), hEnv.HandlerEnvironment)
	sendTelemetry = telemetry.LogStdOutAndEventWithSender(telemetry.NewTelemetryEventSender(eem))
	// check sub-command preconditions, if any, before executing
	sendTelemetry(logger, telemetry.EventLevelInfo, telemetry.MainTask, fmt.Sprintf("Starting AppHealth Extension %s", GetExtensionVersion()))
	sendTelemetry(logger, telemetry.EventLevelInfo, telemetry.MainTask, fmt.Sprintf("HandlerEnviroment = %s", hEnv))
	if cmd.pre != nil {
		logger.Log("event", "pre-check")
		if err := cmd.pre(logger, seqNum); err != nil {
			logger.Log("event", "pre-check failed", "error", err)
			os.Exit(cmd.failExitCode)
		}
	}
	// execute the subcommand
	reportStatus(logger, hEnv, seqNum, StatusTransitioning, cmd, "")
	msg, err := cmd.f(logger, hEnv, seqNum)
	if err != nil {
		logger.Log("event", "failed to handle", "error", err)
		reportStatus(logger, hEnv, seqNum, StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	reportStatus(logger, hEnv, seqNum, StatusSuccess, cmd, msg)
	sendTelemetry(logger, telemetry.EventLevelInfo, telemetry.MainTask, fmt.Sprintf("Finished execution of AppHealth Extension %s", GetExtensionVersion()))
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
