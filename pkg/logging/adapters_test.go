package logging

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	telemetry "github.com/Azure/azure-extension-platform/pkg/extensionevents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoggerAdaptorFromLogger(t *testing.T) {
	// Create a mock ExtensionLogger
	mockLogger, err := NewExtensionLogger(nil)
	require.NoError(t, err, "Failed to create mock logger")

	// Test with a valid ExtensionLogger
	adaptor, err := NewAdaptor(mockLogger)
	assert.NoError(t, err)
	assert.NotNil(t, adaptor)
	assert.Equal(t, mockLogger, adaptor.logger)

	// Test with an invalid logger
	adaptor, err = NewAdaptor(nil)
	assert.Error(t, err)
	assert.Nil(t, adaptor)
}

func TestLoggerAdaptor_PassToEventManager(t *testing.T) {
	// Create a mock ExtensionLogger
	mockLogger, err := NewExtensionLogger(nil)
	require.NoError(t, err, "Failed to create mock logger")
	// Test with a valid ExtensionLogger
	adaptor, err := NewAdaptor(mockLogger)
	assert.NoError(t, err, "Failed to create adaptor Logger")
	assert.NotNil(t, adaptor)
	assert.Equal(t, mockLogger, adaptor.logger)

	var (
		logDir    = "/tmp/logs"
		eventsDir = "/tmp/events"
	)
	// Create the log directory
	err = createDirectories(logDir, eventsDir)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDir, eventsDir)

	fakeEnv := &handlerenv.HandlerEnvironment{
		LogFolder:    logDir,
		EventsFolder: eventsDir,
	}
	eem := telemetry.New(adaptor, fakeEnv)

	// Test the Warn method
	adaptor.Info("This is a test warning message")
	eem.LogInformationalEvent("AppHealth", "This is a test informational event")
}

func TestAdapter_EventLibrary_SideEffects(t *testing.T) {
	// Define the log directory
	var (
		logDir = "/tmp/logs"
	)
	// Create the log directory
	err := createDirectories(logDir)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDir)

	fakeEnv := &handlerenv.HandlerEnvironment{
		HeartbeatFile: "",
		StatusFolder:  "",
		ConfigFolder:  "",
		LogFolder:     logDir,
		DataFolder:    "",
		EventsFolder:  "",
	}

	mockLogger, err := NewExtensionLogger(fakeEnv)
	mockLogger.With("version", "1.0.0", "operation", "test")
	require.NoError(t, err, "Failed to create mock logger")

	// Create a new Adapter Logger
	adaptor, err := NewAdaptor(mockLogger)
	require.NoError(t, err, "Failed to create adaptor Logger")

	eem := telemetry.New(adaptor, fakeEnv)

	duration, _ := time.ParseDuration("100ms")
	eem.LogCriticalEvent("AppHealth", "critical message")
	time.Sleep(duration)

	// Verify that the log file was created
	logFile := adaptor.logger.(*ExtensionLogger).file
	assert.FileExists(t, logFile.Name(), "log file should have been created")

	// Read the log file
	logData, err := os.ReadFile(logFile.Name())
	assert.NoError(t, err)

	// Verify that the log message contains the error
	logOutput := string(logData)
	assert.Contains(t, logOutput, `level=Warning msg="EventsFolder not set. Not writing event."`)

	// Validating each log from the Log ouput, expected format is time=2024-03-12T01:55:55.268Z level=Warning msg="EventsFolder not set. Not writing event." pid=73973
	// We want to also validated slog extra keys like pid, version, tid, operationID
	assert.Contains(t, logOutput, `pid=`)
	assert.Contains(t, logOutput, `version=1.0.0`)
	assert.Contains(t, logOutput, `operation=test`)

	// Verify Adaptor logger has the same logger as the mock logger
	assert.Equal(t, mockLogger, adaptor.logger, "Adaptor logger should have the same logger as the mock logger")

	// Adding more attributes to the logger and validating that adaptor logger has the same attributes
	mockLogger.With("foo", "test")
	eem.LogInformationalEvent("AppHealth", "critical message")
	time.Sleep(duration)

	// Read the log file
	logData, err = os.ReadFile(logFile.Name())
	assert.NoError(t, err)

	// Verify that the log message contains the error
	logOutput = string(logData)
	assert.Contains(t, logOutput, `foo=test`)

}

func TestAdapter_PlatformLogger(t *testing.T) {
	// Define the log directory
	var (
		logDir    = "/tmp/logs"
		eventsDir = "/tmp/events"
	)
	// Create the log directory
	err := createDirectories(logDir, eventsDir)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDir, eventsDir)

	fakeEnv := &handlerenv.HandlerEnvironment{
		HeartbeatFile: "",
		StatusFolder:  "",
		ConfigFolder:  "",
		LogFolder:     logDir,
		DataFolder:    "",
		EventsFolder:  eventsDir,
	}

	mockLogger, err := NewExtensionLogger(nil)
	require.NoError(t, err, "Failed to create mock logger")

	// Create a new Adapter Logger
	adaptor, err := NewAdaptor(mockLogger)
	require.NoError(t, err, "Failed to create adaptor Logger")

	eem := telemetry.New(adaptor, fakeEnv)

	duration, _ := time.ParseDuration("100ms")
	eem.LogCriticalEvent("AppHealth", "critical message")
	time.Sleep(duration)
	eem.LogErrorEvent("VMWatch", "error message")
	time.Sleep(duration)
	eem.LogInformationalEvent("AppHealth", "informational message")
	time.Sleep(duration)
	eem.LogVerboseEvent("GuestHealthPlatform", "verbose message")
	time.Sleep(duration)
	eem.LogWarningEvent("VMWatch", "warning message")

	dir, _ := os.ReadDir(eventsDir)
	require.Equal(t, 5, len(dir))

	verifyEventFile(t, dir[0].Name(), "Critical", "critical message", eventsDir)
	verifyEventFile(t, dir[1].Name(), "Error", "error message", eventsDir)
	verifyEventFile(t, dir[2].Name(), "Informational", "informational message", eventsDir)
	verifyEventFile(t, dir[3].Name(), "Verbose", "verbose message", eventsDir)
	verifyEventFile(t, dir[4].Name(), "Warning", "warning message", eventsDir)
}

func verifyEventFile(t *testing.T, fileName string, expectedLevel string, expectedMessage string, eventsdir string) {
	require.Equal(t, ".json", filepath.Ext(fileName))
	openedFile, err := os.Open(path.Join(eventsdir, fileName))
	require.NoError(t, err)
	defer openedFile.Close()

	b, err := io.ReadAll(openedFile)
	require.NoError(t, err)
	var ee struct {
		Version     string `json:"Version"`
		Timestamp   string `json:"Timestamp"`
		TaskName    string `json:"TaskName"`
		EventLevel  string `json:"EventLevel"`
		Message     string `json:"Message"`
		EventPid    string `json:"EventPid"`
		EventTid    string `json:"EventTid"`
		OperationID string `json:"OperationId"`
	}
	err = json.Unmarshal(b, &ee)
	require.NoError(t, err)

	require.Equal(t, expectedLevel, ee.EventLevel)
	require.Equal(t, expectedMessage, ee.Message)
}
