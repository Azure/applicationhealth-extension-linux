package main

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
	MainTask            EventTask = "Main"
	AppHealthTask       EventTask = "AppHealth"
	AppHealthProbeTask  EventTask = "AppHealth-HealthProbe"
	AppHealthStatusTask EventTask = "AppHealth-Status"
	VMWatchTask         EventTask = "VMWatch"
	VMWatchSetupTask    EventTask = "VMWatch-Setup"
	VMWatchStatusTask   EventTask = "VMWatch-Status"
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

func (t *TelemetryEventSender) sendEvent(level EventLevel, taskName EventTask, message string) {
	switch level {
	case "Critical":
		t.eem.LogCriticalEvent(string(taskName), message)
	case "Error":
		t.eem.LogErrorEvent(string(taskName), message)
	case "Warning":
		t.eem.LogWarningEvent(string(taskName), message)
	case "Verbose":
		t.eem.LogVerboseEvent(string(taskName), message)
	case "Informational":
		t.eem.LogInformationalEvent(string(taskName), message)
	default:
		return
	}
}

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
