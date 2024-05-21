package main

import (
	"fmt"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/go-kit/log"
	"github.com/pkg/errors"
)

// reportStatus saves operation status to the status file for the extension
// handler with the optional given message, if the given cmd requires reporting
// status.
//
// If an error occurs reporting the status, it will be logged and returned.
func reportStatus(lg log.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum int, t StatusType, c cmd, msg string) error {
	if !c.shouldReportStatus {
		lg.Log("status", "not reported for operation (by design)")
		return nil
	}
	s := NewStatus(t, c.name, statusMsg(c, t, msg))
	if err := s.Save(hEnv.StatusFolder, seqNum); err != nil {
		sendTelemetry(lg, telemetry.EventLevelError, telemetry.ReportStatusTask, "failed to save handler status", "error", err.Error())
		return errors.Wrap(err, "failed to save handler status")
	}
	sendTelemetry(lg, telemetry.EventLevelInfo, telemetry.ReportStatusTask, fmt.Sprintf("saved handler status: %s", s))

	return nil
}

func reportStatusWithSubstatuses(lg log.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum int, t StatusType, op string, msg string, substatuses []SubstatusItem) error {
	s := NewStatus(t, op, msg)
	for _, substatus := range substatuses {
		s.AddSubstatusItem(substatus)
	}
	if err := s.Save(hEnv.StatusFolder, seqNum); err != nil {
		sendTelemetry(lg, telemetry.EventLevelError, telemetry.ReportStatusTask, fmt.Sprintf("failed to save handler status: %s", s), "error", err.Error())
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
