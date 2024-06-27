package telemetry

import (
	"runtime"

	"github.com/Azure/azure-extension-platform/pkg/extensionevents"
	"github.com/go-kit/log"
)

type EventLevel string

type EventTask string

const (
	CriticalEvent EventLevel = "Critical"
	ErrorEvent    EventLevel = "Error"
	WarningEvent  EventLevel = "Warning"
	VerboseEvent  EventLevel = "Verbose"
	InfoEvent     EventLevel = "Informational"
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

type Telemetry struct {
	eem *extensionevents.ExtensionEventManager
}

var (
	instance *Telemetry
	once     sync.Once
	mutex    sync.Mutex
)

// LogEvent sends a telemetry event with the specified level, task name, and message.
func (t *Telemetry) SendEvent(level EventLevel, taskName EventTask, message string, keyvals ...interface{}) {
	var (
		eventDispatcher = map[EventLevel]func(string, string){
			CriticalEvent: t.eem.LogCriticalEvent,
			ErrorEvent:    t.eem.LogErrorEvent,
			WarningEvent:  t.eem.LogWarningEvent,
			VerboseEvent:  t.eem.LogVerboseEvent,
			InfoEvent:     t.eem.LogInformationalEvent,
		}
		logDispatcher = map[EventLevel]func(string, ...any){
			CriticalEvent: slog.Error, // Implement Critical log level
			ErrorEvent:    slog.Error,
			WarningEvent:  slog.Warn,
			VerboseEvent:  slog.Debug, // Implement Verbose log level
			InfoEvent:     slog.Info,
		}
	)

	keyvals = append(keyvals, "task", taskName)
	// Select the appropriate event dispatcher and log dispatcher based on the event level.
	// then log and send the event.
	if dispatchFunc, ok := eventDispatcher[level]; ok {
		if log, ok := logDispatcher[level]; ok {
			log(message, keyvals...)
		}
		dispatchFunc(string(taskName), message)
	} else {
		slog.Error("Invalid event level", "level", level)
	}
}

func (t *Telemetry) SetOperationID(operationID string) {
	t.eem.SetOperationID(operationID)
}

}
