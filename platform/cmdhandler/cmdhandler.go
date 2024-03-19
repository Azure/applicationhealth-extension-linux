package cmdhandler

import (
	"fmt"
	"os"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/version"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

type CommandKey string
type CommandName string

func (c CommandName) String() string {
	return string(c)
}

func (c CommandKey) String() string {
	return string(c)
}

const (
	InstallKey   CommandKey = "install"
	UninstallKey CommandKey = "uninstall"
	EnableKey    CommandKey = "enable"
	UpdateKey    CommandKey = "update"
	DisableKey   CommandKey = "disable"
)

const (
	InstallName   CommandName = "Install"
	UninstallName CommandName = "Uninstall"
	EnableName    CommandName = "Enable"
	UpdateName    CommandName = "Update"
	DisableName   CommandName = "Disable"
)

type cmdFunc func(lg logging.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum int) (msg string, err error)
type preFunc func(lg logging.Logger, seqNum int) error

type cmd struct {
	f                  cmdFunc     // associated function
	Name               CommandName // human readable string
	ShouldReportStatus bool        // determines if running this should log to a .status file
	pre                preFunc     // executed before any status is reported
	failExitCode       int         // exitCode to use when commands fail
}

type CommandMap map[CommandKey]cmd

// Get CommandMap Keys as list
func (cm CommandMap) Keys() []CommandKey {
	keys := make([]CommandKey, 0, len(cm))
	for k := range cm {
		keys = append(keys, k)
	}
	return keys
}

// Get CommandMap Values as list
func (cm CommandMap) Values() []cmd {
	values := make([]cmd, 0, len(cm))
	for _, v := range cm {
		values = append(values, v)
	}
	return values
}

type CommandHandler interface {
	Execute(lg logging.Logger, c CommandKey, h *handlerenv.HandlerEnvironment, seqNum int) error
	CommandMap() CommandMap
}

// returns a new CommandHandler depending on the OS
func NewCommandHandler() (CommandHandler, error) {
	handler, err := newOSCommandHandler()
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func noop(lg logging.Logger, h *handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	lg.Info("noop")
	return "", nil
}

// parseCmd looks at os.Args and parses the subcommand. If it is invalid,
// it prints the usage string and an error message and exits with code 0.
func ParseCmd() (CommandKey, error) {
	if len(os.Args) != 2 {
		printUsage(os.Args)
		return "", fmt.Errorf("Incorrect usage")
	}
	op := os.Args[1]
	// Check if the command is valid key defined in CommandKeys
	switch op {
	case InstallKey.String(), UninstallKey.String(), EnableKey.String(), UpdateKey.String(), DisableKey.String():
		return CommandKey(op), nil
	default:
		printUsage(os.Args)
		return "", fmt.Errorf("Incorrect command: %q\n", op)
	}
}

// printUsage prints the help string and version of the program to stdout with a
// trailing new line.
func printUsage(args []string) {
	fmt.Printf("Usage: %s ", os.Args[0])
	i := 0
	for k := range extCommands {
		if i > 0 {
			fmt.Print("|")
		}
		fmt.Print(k.String())
		i++
	}
	fmt.Println()
	fmt.Println(version.DetailedVersionString())
}
