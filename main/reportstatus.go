package main

import (
	"github.com/Azure/azure-docker-extension/pkg/vmextension"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

// reportStatus saves operation status to the status file for the extension
// handler with the optional given message, if the given cmd requires reporting
// status.
//
// If an error occurs reporting the status, it will be logged and returned.
func reportStatus(ctx *log.Context, hEnv vmextension.HandlerEnvironment, seqNum int, t StatusType, c cmd, msg string) error {
	if !c.shouldReportStatus {
		ctx.Log("status", "not reported for operation (by design)")
		return nil
	}
	s := NewStatus(t, c.name, statusMsg(c, t, msg))
	if err := s.Save(hEnv.HandlerEnvironment.StatusFolder, seqNum); err != nil {
		ctx.Log("event", "failed to save handler status", "error", err)
		return errors.Wrap(err, "failed to save handler status")
	}
	return nil
}

func reportStatusWithSubstatus(ctx *log.Context, hEnv vmextension.HandlerEnvironment, seqNum int, t StatusType, op string, msg string, subType StatusType, subName string, subMessage string) error {
	s := NewStatus(t, op, msg)
	s.AddSubstatus(subType, subName, subMessage)
	if err := s.Save(hEnv.HandlerEnvironment.StatusFolder, seqNum); err != nil {
		ctx.Log("event", "failed to save handler status", "error", err)
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