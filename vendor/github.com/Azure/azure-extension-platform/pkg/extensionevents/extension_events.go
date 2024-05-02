// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
package extensionevents

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/Azure/azure-extension-platform/pkg/handlerenv"
	"github.com/Azure/azure-extension-platform/pkg/logging"
)

const (
	eventLevelCritical      = "Critical"
	eventLevelError         = "Error"
	eventLevelWarning       = "Warning"
	eventLevelVerbose       = "Verbose"
	eventLevelInformational = "Informational"
)

type extensionEvent struct {
	Version     string `json:"Version"`
	Timestamp   string `json:"Timestamp"`
	TaskName    string `json:"TaskName"`
	EventLevel  string `json:"EventLevel"`
	Message     string `json:"Message"`
	EventPid    string `json:"EventPid"`
	EventTid    string `json:"EventTid"`
	OperationID string `json:"OperationId"`
}

// ExtensionEventManager allows extensions to log events that will be collected
// by the Guest Agent
type ExtensionEventManager struct {
	extensionLogger logging.ILogger
	eventsFolder    string
	operationID     string
}

func (eem *ExtensionEventManager) logEvent(taskName string, eventLevel string, message string) {
	if eem.eventsFolder == "" {
		eem.extensionLogger.Warn("EventsFolder not set. Not writing event.")
		return
	}

	extensionVersion := os.Getenv("AZURE_GUEST_AGENT_EXTENSION_VERSION")
	timestamp := time.Now().UTC().Format(time.RFC3339)
	pid := fmt.Sprintf("%v", os.Getpid())
	tid := getThreadID()

	extensionEvent := extensionEvent{
		Version:     extensionVersion,
		Timestamp:   timestamp,
		TaskName:    taskName,
		EventLevel:  eventLevel,
		Message:     message,
		EventPid:    pid,
		EventTid:    tid,
		OperationID: eem.operationID,
	}

	// File name is the unix time in milliseconds
	fileName := strconv.FormatInt(time.Now().UTC().UnixNano()/1000, 10)
	filePath := path.Join(eem.eventsFolder, fileName) + ".json"

	b, err := json.Marshal(extensionEvent)
	if err != nil {
		eem.extensionLogger.Error("Unable to serialize extension event: <%v>", err)
		return
	}

	err = ioutil.WriteFile(filePath, b, 0644)
	if err != nil {
		eem.extensionLogger.Error("Unable to write event file: <%v>", err)
	}
}

// New creates a new instance of the ExtensionEventManager
func New(el logging.ILogger, he *handlerenv.HandlerEnvironment) *ExtensionEventManager {
	eem := &ExtensionEventManager{
		extensionLogger: el,
		eventsFolder:    he.EventsFolder,
		operationID:     "",
	}

	return eem
}

// "SetOperationId()" sets operation Id passed by user while logging extension events
// This is made as separate function (not included in "logEvent()") to enable users to set Operation ID globally for their extension.
// "operationID" corresponds to "Context3" column in 'GuestAgentGenericLogs' table (Rdos cluster)
func (eem *ExtensionEventManager) SetOperationID(operationID string) {
	eem.operationID = operationID
}

// LogCriticalEvent writes a message with critical status for the extension
func (eem *ExtensionEventManager) LogCriticalEvent(taskName string, message string) {
	eem.logEvent(taskName, eventLevelCritical, message)
}

// LogErrorEvent writes a message with error status for the extension
func (eem *ExtensionEventManager) LogErrorEvent(taskName string, message string) {
	eem.logEvent(taskName, eventLevelError, message)
}

// LogWarningEvent writes a message with warning status for the extension
func (eem *ExtensionEventManager) LogWarningEvent(taskName string, message string) {
	eem.logEvent(taskName, eventLevelWarning, message)
}

// LogVerboseEvent writes a message with verbose status for the extension
func (eem *ExtensionEventManager) LogVerboseEvent(taskName string, message string) {
	eem.logEvent(taskName, eventLevelVerbose, message)
}

// LogInformationalEvent writes a message with informational status for the extension
func (eem *ExtensionEventManager) LogInformationalEvent(taskName string, message string) {
	eem.logEvent(taskName, eventLevelInformational, message)
}
