package telemetry

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/azure-extension-platform/pkg/extensionevents"
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

var (
	ErrUnableToInitialize = fmt.Errorf("unable to initialize telemetry")
	ErrTelemetryNotInit   = fmt.Errorf("telemetry not initialized")
)

type Telemetry struct {
	eem *extensionevents.ExtensionEventManager
}

var (
	instance *Telemetry
	once     sync.Once
	mutex    sync.Mutex
)

func NewTelemetry(h *handlerenv.HandlerEnvironment) (*Telemetry, error) {
	if instance != nil {
		slog.Warn("Telemetry instance already initialized")
		return instance, nil
	}
	if h.EventsFolder == "" {
		return nil, fmt.Errorf("events folder is not set: %w", ErrUnableToInitialize)
	}
	once.Do(func() {
		instance = &Telemetry{
			eem: extensionevents.New(logging.NewNopLogger(), &h.HandlerEnvironment),
		}
	})
	return instance, nil
}

func GetTelemetry() (*Telemetry, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if instance == nil {
		return nil, ErrTelemetryNotInit
	}

	return instance, nil
}

// LogEvent sends a telemetry event with the specified level, task name, and message.
func (t *Telemetry) SendEvent(level EventLevel, taskName EventTask, message string, keyvals ...interface{}) {
	keyvals = append(keyvals, "task", taskName)
	// Select the appropriate event dispatcher and log dispatcher based on the event level.
	// then log and send the event.
	if eventDispatcher, ok := t.getEventDispatcherFunc(level); ok {
		if log, ok := t.getLogDispatcherFunc(level); ok {
			log(message, keyvals...)
		}
		eventDispatcher(string(taskName), message)
	} else {
		slog.Error("Invalid event level", "level", level)
	}
}

// Helper method to dynamically get the dispatch function
func (t *Telemetry) getEventDispatcherFunc(level EventLevel) (func(string, string), bool) {
	switch level {
	case InfoEvent:
		return t.eem.LogInformationalEvent, true
	case VerboseEvent:
		return t.eem.LogVerboseEvent, true
	case WarningEvent:
		return t.eem.LogWarningEvent, true
	case ErrorEvent:
		return t.eem.LogErrorEvent, true
	case CriticalEvent:
		return t.eem.LogCriticalEvent, true
	default:
		return nil, false
	}
}

func (t *Telemetry) getLogDispatcherFunc(level EventLevel) (func(string, ...any), bool) {
	switch level {
	case InfoEvent:
		return slog.Info, true
	case VerboseEvent:
		return slog.Debug, true
	case WarningEvent:
		return slog.Warn, true
	case ErrorEvent:
		return slog.Error, true
	case CriticalEvent:
		return slog.Error, true
	default:
		return nil, false
	}
}

func (t *Telemetry) SetOperationID(operationID string) {
	t.eem.SetOperationID(operationID)
}

// SendEvent sends an event with the specified level, task name, message, and key-value pairs.
// It is a package level function that can be used to send telemetry events.
// If the instance is nil, the function returns without sending the event.
func SendEvent(level EventLevel, taskName EventTask, message string, keyvals ...interface{}) {
	if instance == nil {
		return
	}
	instance.SendEvent(level, taskName, message, keyvals...)
}
