package main

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
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

var (
	// dataDir is where we store the logs and state for the extension handler
	dataDir = "/var/lib/waagent/apphealth"

	shutdown = false

	// We need a reference to the command here so that we can cleanly shutdown VMWatch process
	// when a shutdown signal is received
	vmWatchCommand *exec.Cmd
	// the logger that will be used throughout
	lg = logging.NewExtensionLogger(nil)
)

func main() {

	lg.With("version", VersionString())

	// parse command line arguments
	cmd := parseCmd(os.Args)

	h, err := handlerenv.GetHandlerEnviroment()
	if err != nil {
		lg.Error("failed to parse Handler Enviroment", slog.Any("error", err))
		os.Exit(cmd.failExitCode)
	}
	// Creating new Logger with the handler environment
	lg = logging.NewExtensionLogger(&h)
	lg.With("version", VersionString())
	lg.With("operation", strings.ToLower(cmd.name))

	seqNum, err := FindSeqNum(h.HandlerEnvironment.ConfigFolder)
	if err != nil {
		lg.Error("failed to find sequence number", slog.Any("error", err))
	}

	lg.With("seq", strconv.Itoa(seqNum))

	lg.Info("Starting AppHealth Extension")
	lg.Info(fmt.Sprintf("HandlerEnvironment: %v", h))

	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		lg.Info("Received shutdown request")
		shutdown = true
		err := killVMWatch(lg, vmWatchCommand)
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
	reportStatus(lg, h, seqNum, StatusTransitioning, cmd, "")
	msg, err := cmd.f(lg, h, seqNum)
	if err != nil {
		lg.Error("failed to handle", slog.Any("error", err))
		reportStatus(lg, h, seqNum, StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	reportStatus(lg, h, seqNum, StatusSuccess, cmd, msg)
	lg.Info("end")
}

// parseCmd looks at os.Args and parses the subcommand. If it is invalid,
// it prints the usage string and an error message and exits with code 0.
func parseCmd(args []string) cmd {
	if len(os.Args) != 2 {
		printUsage(args)
		err := fmt.Errorf("failed to parse command line arguments, expected 2, got %d", len(os.Args))
		lg.Error("Incorrect usage", slog.Any("error", err))
		os.Exit(2)
	}
	op := os.Args[1]
	cmd, ok := cmds[op]
	if !ok {
		printUsage(args)
		err := fmt.Errorf("failed to parse command line arguments, command not found: %s", op)
		lg.Error(fmt.Sprintf("Incorrect command: %q\n", op), slog.Any("error", err))
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
