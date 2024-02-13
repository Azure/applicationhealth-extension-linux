package main

import (
	"fmt"
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
	lg = logging.New(nil)
)

func main() {

	lg.WithProcessID()
	lg.WithTimestamp()
	lg.WithVersion(VersionString())

	// parse command line arguments
	cmd := parseCmd(os.Args)

	hEnv, err := handlerenv.GetHandlerEnviroment()
	if err != nil {
		lg.EventError("failed to parse Handler Enviroment", err)
		os.Exit(cmd.failExitCode)
	}
	// Creating new Logger with the handler environment
	lg = logging.New(&hEnv)
	lg.WithProcessID()
	lg.WithTimestamp()
	lg.WithVersion(VersionString())
	lg.WithOperation(strings.ToLower(cmd.name))

	seqNum, err := FindSeqNum(hEnv.HandlerEnvironment.ConfigFolder)
	if err != nil {
		lg.EventError("failed to find sequence number", err)
	}

	lg.WithSeqNum(strconv.Itoa(seqNum))

	// check sub-command preconditions, if any, before executing
	lg.Event("Starting AppHealth Extension")
	lg.Event(fmt.Sprintf("HandlerEnvironment: %v", hEnv))

	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		lg.Event("Received shutdown request")
		shutdown = true
		err := killVMWatch(*lg, vmWatchCommand)
		if err != nil {
			lg.EventError("error when killing vmwatch", err)
		}
	}()
	if cmd.pre != nil {
		lg.Event("pre-check")
		if err := cmd.pre(*lg, seqNum); err != nil {
			lg.EventError("pre-check failed", err)
			os.Exit(cmd.failExitCode)
		}
	}
	// execute the subcommand
	reportStatus(*lg, hEnv, seqNum, StatusTransitioning, cmd, "")
	msg, err := cmd.f(*lg, hEnv, seqNum)
	if err != nil {
		lg.EventError("failed to handle", err)
		reportStatus(*lg, hEnv, seqNum, StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	reportStatus(*lg, hEnv, seqNum, StatusSuccess, cmd, msg)
	lg.Event("end")
}

// parseCmd looks at os.Args and parses the subcommand. If it is invalid,
// it prints the usage string and an error message and exits with code 0.
func parseCmd(args []string) cmd {
	if len(os.Args) != 2 {
		printUsage(args)
		lg.EventError("Incorrect usage")
		os.Exit(2)
	}
	op := os.Args[1]
	cmd, ok := cmds[op]
	if !ok {
		printUsage(args)
		lg.EventError(fmt.Sprintf("Incorrect command: %q\n", op))
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
