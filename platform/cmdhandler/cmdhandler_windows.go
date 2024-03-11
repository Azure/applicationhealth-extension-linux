package cmdhandler

import (
	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
)

var extCommands CommandMap = nil // TODO: Implement

type WindowsCommandHandler struct {
	commands CommandMap
}

func newOSCommandHandler() (CommandHandler, error) {
	return &WindowsCommandHandler{
		commands: nil, // TODO: Implement
	}, nil
}

func (*WindowsCommandHandler) Execute(lg logging.Logger, c CommandKey, h *handlerenv.HandlerEnvironment, seqNum int) error {
	// TODO: Implement command execution
	return nil
}

func (ch *WindowsCommandHandler) CommandMap() CommandMap {
	return ch.commands
}
