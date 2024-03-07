package cmdhandler

import (
	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/pkg/errors"
)

var extCommands CommandMap = nil // TODO: Implement

type WindowsCommandHandler struct {
	commands CommandMap
	target   CommandKey
}

func newOSCommandHandler() (CommandHandler, error) {
	return &WindowsCommandHandler{
		commands: nil, // TODO: Implement
	}, nil
}

func (*WindowsCommandHandler) Execute(h handlerenv.HandlerEnvironment, seqNum int) error {
	// TODO: Implement command execution
	return nil
}

func (ch *WindowsCommandHandler) CommandMap() CommandMap {
	return ch.commands
}

func (ch *WindowsCommandHandler) SetCommandToExecute(key CommandKey) error {
	// TODO: Implement Correctly for Windows
	if _, ok := ch.commands[key]; !ok {
		return errors.Errorf("unknown command: %s", key)
	}
	ch.target = key
	return nil
}
