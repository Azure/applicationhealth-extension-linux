package logging

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Test creating a logger with a handler environment
	fakeEnv := &handlerenv.HandlerEnvironment{}
	fakeEnv.HandlerEnvironment.LogFolder = "/tmp/logs"
	logger := New(fakeEnv)
	defer logger.Close()

	assert.NotNil(t, logger)
}

func TestNewWithName(t *testing.T) {
	// Test creating a logger with a handler environment and custom log file name format
	fakeEnv := &handlerenv.HandlerEnvironment{}
	fakeEnv.HandlerEnvironment.LogFolder = "/tmp/logs"
	logger := NewWithName(fakeEnv, "log_%v_test")
	defer logger.Close()

	assert.NotNil(t, logger)
}

func TestExtensionLogger_Event(t *testing.T) {
	// Define the log directory
	logDir := "/tmp/logs"

	// Create the log directory
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// Ensure the log directory is deleted even if the test fails
	defer os.RemoveAll(logDir)

	// Test logging an event with a named logger
	fakeEnv := &handlerenv.HandlerEnvironment{}
	fakeEnv.HandlerEnvironment.LogFolder = logDir
	logger := NewWithName(fakeEnv, "log_%v_test")
	defer logger.Close()

	// Write a log message
	logger.Event("test event")

	// Get the log file name
	logFileName := logger.file

	// Read the log file
	logData, err := ioutil.ReadFile(logFileName.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Verify that the log message contains the event
	logOutput := string(logData)

	assert.Contains(t, logOutput, "event")
	assert.Contains(t, logOutput, "test event")
}

func TestRotateLogFolder(t *testing.T) {
	// Define the log directory
	// Create a temporary log folder for testing
	logFolder, err := ioutil.TempDir("", "logs")
	assert.NoError(t, err)
	defer os.RemoveAll(logFolder)

	// Create some log files in the log folder
	logFile1 := filepath.Join(logFolder, "log_1")
	logFile2 := filepath.Join(logFolder, "log_2")
	logFile3 := filepath.Join(logFolder, "log_3")

	// Generate a large amount of data to write to the log files
	largeData := make([]byte, 15*1024*1024) // 15 MB
	for i := range largeData {
		largeData[i] = 'A'
	}

	err = ioutil.WriteFile(logFile1, largeData, 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(logFile2, largeData, 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(logFile3, largeData, 0644)
	assert.NoError(t, err)

	// Rotate the log folder
	err = rotateLogFolder(logFolder, "log_%v")
	assert.NoError(t, err)

	// Verify that only the newest log file remains
	_, err = os.Stat(logFile1)
	assert.Error(t, err, "log_1 should have been deleted")
	_, err = os.Stat(logFile2)
	assert.NoError(t, err, "log_2 should not have been deleted")
	_, err = os.Stat(logFile3)
	assert.NoError(t, err, "log_3 should not have been deleted")
}

func TestRotateLogFolder_DirectorySizeBelowThreshold(t *testing.T) {
	// Create a temporary log folder for testing
	logFolder, err := ioutil.TempDir("", "logs")
	assert.NoError(t, err)
	defer os.RemoveAll(logFolder)

	// Create some log files in the log folder
	logFile1 := filepath.Join(logFolder, "log_1")
	logFile2 := filepath.Join(logFolder, "log_2")
	logFile3 := filepath.Join(logFolder, "log_3")

	// Generate a large amount of data to write to the log files
	largeData := make([]byte, 15*1024*1024) // 15 MB
	for i := range largeData {
		largeData[i] = 'A'
	}

	err = ioutil.WriteFile(logFile1, []byte("log file 1"), 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(logFile2, largeData, 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(logFile3, largeData, 0644)
	assert.NoError(t, err)

	// Rotate the log folder when the directory size is below the threshold
	err = rotateLogFolder(logFolder, "log_%v")
	assert.NoError(t, err)

	// Verify that only the newest log file remains
	_, err = os.Stat(logFile1)
	assert.NoError(t, err, "log_1 should not have been deleted")
	_, err = os.Stat(logFile2)
	assert.NoError(t, err, "log_2 should not have been deleted")
	_, err = os.Stat(logFile3)
	assert.NoError(t, err, "log_3 should not have been deleted")
}
