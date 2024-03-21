package cmdhandler

import (
	"log/slog"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/applicationhealth-extension-linux/pkg/status"
	"github.com/pkg/errors"
)

// reportStatus saves operation status to the status file for the extension
// handler with the optional given message, if the given cmd requires reporting
// status.
//
// If an error occurs reporting the status, it will be logged and returned.
func ReportStatus(lg logging.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum int, t status.StatusType, c cmd, msg string) error {
	if !c.ShouldReportStatus {
		lg.Info("status not reported for operation (by design)")
		return nil
	}
	s := status.NewStatus(t, c.Name.String(), statusMsg(c, t, msg))
	if err := s.Save(hEnv.StatusFolder, seqNum); err != nil {
		lg.Error("failed to save handler status", slog.Any("error", err))
		return errors.Wrap(err, "failed to save handler status")
	}
	return nil
}

func ReportStatusWithSubstatuses(lg logging.Logger, hEnv *handlerenv.HandlerEnvironment, seqNum int, t status.StatusType, op string, msg string, substatuses []status.SubstatusItem) error {
	s := status.NewStatus(t, op, msg)
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
func statusMsg(c cmd, t status.StatusType, msg string) string {
	s := c.Name.String()
	switch t {
	case status.StatusSuccess:
		s += " succeeded"
	case status.StatusTransitioning:
		s += " in progress"
	case status.StatusError:
		s += " failed"
	}

	if msg != "" {
		// append the original
		s += ": " + msg
	}
	return s
}
