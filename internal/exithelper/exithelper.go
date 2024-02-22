package exithelper

import "os"

type IExitHelper interface {
	Exit(int)
}

const (
	MiscError          = 1
	ArgumentError      = 2
	EnvironmentError   = 3
	CommunicationError = 4
	FileSystemError    = 5
	ExecutionError     = 6
)

type ExitHelper struct{}

func (*ExitHelper) Exit(exitCode int) {
	os.Exit(exitCode)
}

var Exiter IExitHelper = &ExitHelper{}
