package main

import (
	"log/slog"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/pkg/errors"
)

// reportStatus saves operation status to the status file for the extension
// handler with the optional given message, if the given cmd requires reporting
// status.
//
// If an error occurs reporting the status, it will be logged and returned.
func reportStatus(lg logging.Logger, hEnv handlerenv.HandlerEnvironment, seqNum int, t StatusType, c cmd, msg string) error {
	if !c.shouldReportStatus {
		lg.Info("status not reported for operation (by design)")
		return nil
	}
	s := NewStatus(t, c.name, statusMsg(c, t, msg))
	if err := s.Save(hEnv.StatusFolder, seqNum); err != nil {
		lg.Error("failed to save handler status", slog.Any("error", err))
		return errors.Wrap(err, "failed to save handler status")
	}
	return nil
}

func reportStatusWithSubstatuses(lg logging.Logger, hEnv handlerenv.HandlerEnvironment, seqNum int, t StatusType, op string, msg string, substatuses []SubstatusItem) error {
	s := NewStatus(t, op, msg)
	for _, substatus := range substatuses {
		s.AddSubstatusItem(substatus)
	}
	if err := s.Save(hEnv.StatusFolder, seqNum); err != nil {
		lg.Error("failed to save handler status", slog.Any("error", err))
		return errors.Wrap(err, "failed to save handler status")
	}
	return nil
}

// statusMsg creates the reported status message based on the provided operation
// type and the given message string.
//
// A message will be generated for empty string. For error status, pass the
// error message.
func statusMsg(c cmd, t StatusType, msg string) string {
	s := c.name
	switch t {
	case StatusSuccess:
		s += " succeeded"
	case StatusTransitioning:
		s += " in progress"
	case StatusError:
		s += " failed"
	}

	if msg != "" {
		// append the original
		s += ": " + msg
	}
	return s
}
