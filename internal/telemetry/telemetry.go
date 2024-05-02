package telemetry

import (
	"runtime"

	"github.com/Azure/azure-extension-platform/pkg/extensionevents"
	"github.com/go-kit/log"
)

type EventLevel string

type EventTask string

const (
	EventLevelCritical EventLevel = "Critical"
	EventLevelError    EventLevel = "Error"
	EventLevelWarning  EventLevel = "Warning"
	EventLevelVerbose  EventLevel = "Verbose"
	EventLevelInfo     EventLevel = "Informational"
)

const (
	MainTask           EventTask = "Main"
	AppHealthTask      EventTask = "AppHealth"
	AppHealthProbeTask EventTask = "AppHealth-HealthProbe"
	ReportStatusTask   EventTask = "ReportStatus"
	ReportHeatBeatTask EventTask = "CheckHealthAndReportHeartBeat"
	StartVMWatchTask   EventTask = "StartVMWatchIfApplicable"
	StopVMWatchTask    EventTask = "OnExited"
	SetupVMWatchTask   EventTask = "SetupVMWatchProcess"
	KillVMWatchTask    EventTask = "KillVMWatchIfApplicable"
)

type LogFunc func(logger log.Logger, keyvals ...interface{})
type LogEventFunc func(logger log.Logger, level EventLevel, taskName EventTask, message string, keyvals ...interface{})

type TelemetryEventSender struct {
	eem *extensionevents.ExtensionEventManager
}

func NewTelemetryEventSender(eem *extensionevents.ExtensionEventManager) *TelemetryEventSender {
	return &TelemetryEventSender{
		eem: eem,
	}
}

// sendEvent sends a telemetry event with the specified level, task name, and message.
func (t *TelemetryEventSender) sendEvent(level EventLevel, taskName EventTask, message string) {
	switch level {
	case EventLevelCritical:
		t.eem.LogCriticalEvent(string(taskName), message)
	case EventLevelError:
		t.eem.LogErrorEvent(string(taskName), message)
	case EventLevelWarning:
		t.eem.LogWarningEvent(string(taskName), message)
	case EventLevelVerbose:
		t.eem.LogVerboseEvent(string(taskName), message)
	case EventLevelInfo:
		t.eem.LogInformationalEvent(string(taskName), message)
	default:
		return
	}
}

// LogStdOutAndEventWithSender is a higher-order function that returns a LogEventFunc.
// It logs the event to the provided logger and sends the event to the specified sender.
// If the taskName is empty, it automatically determines the caller's function name as the taskName.
// The event level, task name, and message are appended to the keyvals slice.
// Finally, it calls the sender's sendEvent method to send the event.
func LogStdOutAndEventWithSender(sender *TelemetryEventSender) LogEventFunc {
	return func(logger log.Logger, level EventLevel, taskName EventTask, message string, keyvals ...interface{}) {
		if taskName == "" {
			pc, _, _, _ := runtime.Caller(1)
			callerName := runtime.FuncForPC(pc).Name()
			taskName = EventTask(callerName)
		}
		keyvals = append(keyvals, "level", level, "task", taskName, "event", message)
		logger.Log(keyvals...)
		// logger.Log("eventLevel", level, "eventTask", taskName, "event", message)
		(*sender).sendEvent(level, taskName, message)
	}
}
