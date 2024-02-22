package main

import (
	"fmt"
	"log/slog"
	"os"

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
		lg.Error("failed to execute command", err)
	}
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
	fmt.Println(version.DetailedVersionString())
}
