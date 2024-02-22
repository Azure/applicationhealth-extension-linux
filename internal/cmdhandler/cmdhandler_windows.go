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

func (h *WindowsCommandHandler) Execute(h handlerenv.HandlerEnvironment, seqNum int) error {
	// TODO: Implement command execution
	return nil
}

func (h *WindowsCommandHandler) CommandMap() CommandMap {
	return h.commands
}

func (h *WindowsCommandHandler) SetCommandToExecute(key CommandKey) error {
	// TODO: Implement Correctly for Windows
	if _, ok := h.commands[key]; !ok {
		return errors.Errorf("unknown command: %s", cmd)
	}
	h.target = key
	return nil
}
