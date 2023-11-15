package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-kit/kit/log"
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
	ctx := log.NewContext(log.NewSyncLogger(log.NewLogfmtLogger(
		os.Stdout))).With("time", log.DefaultTimestamp).With("version", VersionString())

	// parse command line arguments
	cmd := parseCmd(os.Args)
	ctx = ctx.With("operation", strings.ToLower(cmd.name))

	// subscribe to cleanly shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		ctx.Log("event", fmt.Sprintf("Received shutdown request"))
		shutdown = true
		killVMWatch(ctx, vmWatchCommand)
	}()

	// parse extension environment
	hEnv, err := GetHandlerEnv()
	if err != nil {
		ctx.Log("message", "failed to parse handlerenv", "error", err)
		os.Exit(cmd.failExitCode)
	}
	seqNum, err := FindSeqNum(hEnv.HandlerEnvironment.ConfigFolder)
	if err != nil {
		ctx.Log("messsage", "failed to find sequence number", "error", err)
	}
	ctx = ctx.With("seq", seqNum)

	// check sub-command preconditions, if any, before executing
	ctx.Log("event", "start")
	if cmd.pre != nil {
		ctx.Log("event", "pre-check")
		if err := cmd.pre(ctx, seqNum); err != nil {
			ctx.Log("event", "pre-check failed", "error", err)
			os.Exit(cmd.failExitCode)
		}
	}
	// execute the subcommand
	reportStatus(ctx, hEnv, seqNum, StatusTransitioning, cmd, "")
	msg, err := cmd.f(ctx, hEnv, seqNum)
	if err != nil {
		ctx.Log("event", "failed to handle", "error", err)
		reportStatus(ctx, hEnv, seqNum, StatusError, cmd, err.Error()+msg)
		os.Exit(cmd.failExitCode)
	}
	reportStatus(ctx, hEnv, seqNum, StatusSuccess, cmd, msg)
	ctx.Log("event", "end")
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
