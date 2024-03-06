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
	Install   CommandKey = "install"
	Uninstall CommandKey = "uninstall"
	Enable    CommandKey = "enable"
	Update    CommandKey = "update"
	Disable   CommandKey = "disable"
)

const (
	InstallName   CommandName = "Install"
	UninstallName CommandName = "Uninstall"
	EnableName    CommandName = "Enable"
	UpdateName    CommandName = "Update"
	DisableName   CommandName = "Disable"
)

type cmdFunc func(ctx logging.Logger, hEnv handlerenv.HandlerEnvironment, seqNum int) (msg string, err error)
type preFunc func(ctx logging.Logger, seqNum int) error

type Cmd struct {
	f                  cmdFunc     // associated function
	Name               CommandName // human readable string
	ShouldReportStatus bool        // determines if running this should log to a .status file
	pre                preFunc     // executed before any status is reported
	failExitCode       int         // exitCode to use when commands fail
}

type CommandMap map[CommandKey]Cmd

// Get CommandMap Keys as list
func (cm CommandMap) Keys() []CommandKey {
	keys := make([]CommandKey, 0, len(cm))
	for k := range cm {
		keys = append(keys, k)
	}
	return keys
}

// Get CommandMap Values as list
func (cm CommandMap) Values() []Cmd {
	values := make([]Cmd, 0, len(cm))
	for _, v := range cm {
		values = append(values, v)
	}
	return values
}

type CommandHandler interface {
	Execute(h handlerenv.HandlerEnvironment, seqNum int) error
	CommandMap() CommandMap
	SetCommandToExecute(CommandKey) error
}

// returns a new CommandHandler depending on the OS
func NewCommandHandler() (CommandHandler, error) {
	handler, err := newOSCommandHandler()
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func noop(ctx logging.Logger, h handlerenv.HandlerEnvironment, seqNum int) (string, error) {
	ctx.Info("noop")
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
	case Install.String(), Uninstall.String(), Enable.String(), Update.String(), Disable.String():
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
		fmt.Printf(k.String())
		if i != len(extCommands)-1 {
			fmt.Printf("|")
		}
		i++
	}
	fmt.Println()
	fmt.Println(version.DetailedVersionString())
}
