// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package exithelper

import "os"

const (
	MiscError          = 1
	ArgumentError      = 2
	EnvironmentError   = 3
	CommunicationError = 4
	FileSystemError    = 5
	ExecutionError     = 6
)

type IExitHelper interface {
	Exit(int)
}

type ExitHelper struct{}

func (*ExitHelper) Exit(exitCode int) {
	os.Exit(exitCode)
}

var Exiter IExitHelper = &ExitHelper{}
