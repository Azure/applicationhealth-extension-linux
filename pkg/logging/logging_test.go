package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mb = 1024 * 1024 // 1 MB
)

func TestNew(t *testing.T) {
	// Test creating a logger with a handler environment
	var (
		fakeEnv        = &handlerenv.HandlerEnvironment{}
		logFolder, err = os.MkdirTemp("", "logs")
	)
	require.NoError(t, err)
	defer os.RemoveAll(logFolder)
	fakeEnv.LogFolder = logFolder

	logger, err := NewExtensionLogger(fakeEnv)
	require.NoError(t, err, "Failed to create logger")
	defer logger.Close()

	assert.NotNil(t, logger)
}

func TestNewWithName_Success(t *testing.T) {
	logFolder, err := os.MkdirTemp("", "logs")
	require.NoError(t, err)
	logger, err := NewExtensionLoggerWithName(logFolder, "log_%v_test")
	require.NoError(t, err, "Failed to create logger")
	require.NotNil(t, logger, "Logger should not be nil")
}

func TestNewWithName_NoDirExist(t *testing.T) {
	// Test creating a logger with a handler environment and custom log file name format
	var (
		logDir      = "/tmp/logs"
		logger, err = NewExtensionLoggerWithName(logDir, "log_%v_test")
	)
	require.NoDirExists(t, logDir, "Log directory should not have been created")
	require.Error(t, err, "Failed to create logger")
	assert.Nil(t, logger, "Logger should be nil")
}

func TestRotateLogFolder(t *testing.T) {
	// Create a temporary log folder for testing
	logFolder, err := os.MkdirTemp("", "logs")
	assert.NoError(t, err)
	defer os.RemoveAll(logFolder)

	// Create some log files in the log folder
	logFile1 := filepath.Join(logFolder, "log_1")
	logFile2 := filepath.Join(logFolder, "log_2")
	logFile3 := filepath.Join(logFolder, "log_3")

	// Generate a large amount of data to write to the log files
	largeData := make([]byte, 14*mb) // 14 MB
	for i := range largeData {
		largeData[i] = 'A'
	}

	err = os.WriteFile(logFile1, largeData, 0644)
	assert.NoError(t, err)
	err = os.WriteFile(logFile2, largeData, 0644)
	assert.NoError(t, err)
	err = os.WriteFile(logFile3, largeData, 0644)
	assert.NoError(t, err)

	// Created 3 files with 14MB data each, total 42MB,
	// which is greater than the threshold of 40MB. Only one file should remain after rotation.
	// because the threshold lowbound is 30MB, and after deleting the oldest file, the total size will be 28MB.
	// Rotate the log folder
	err = rotateLogFolder(logFolder, "log_%v")
	assert.NoError(t, err)

	// Verify that only the newest log file remains
	_, err = os.Stat(logFile1)
	assert.Error(t, err, "log_1 should have been deleted")
	assert.NoFileExistsf(t, logFile1, "Log File: %s should have been deleted by file rotation", logFile1)
	_, err = os.Stat(logFile2)
	assert.NoError(t, err, "log_2 should not have been deleted")
	assert.FileExistsf(t, logFile2, "Log File: %s should exist", logFile2)
	_, err = os.Stat(logFile3)
	assert.NoError(t, err, "log_3 should not have been deleted")
	assert.FileExistsf(t, logFile3, "Log File: %s should exist", logFile3)
}

func TestRotateLogFolder_DirectorySizeBelowThreshold(t *testing.T) {
	// Create a temporary log folder for testing
	logFolder, err := os.MkdirTemp("", "logs")
	assert.NoError(t, err)
	defer os.RemoveAll(logFolder)

	// Create some log files in the log folder
	logFile1 := filepath.Join(logFolder, "log_1")
	logFile2 := filepath.Join(logFolder, "log_2")
	logFile3 := filepath.Join(logFolder, "log_3")

	// Generate a large amount of data to write to the log files
	largeData := make([]byte, 15*mb) // 15 MB
	for i := range largeData {
		largeData[i] = 'A'
	}

	err = os.WriteFile(logFile1, []byte("log file 1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(logFile2, largeData, 0644)
	assert.NoError(t, err)
	err = os.WriteFile(logFile3, largeData, 0644)
	assert.NoError(t, err)

	// Rotate the log folder when the directory size is below the threshold
	err = rotateLogFolder(logFolder, "log_%v")
	assert.NoError(t, err)

	// Verify that no log files were deleted
	_, err = os.Stat(logFile1)
	assert.NoError(t, err, "log_1 should not have been deleted")
	assert.FileExistsf(t, logFile1, "Log File: %s should exist", logFile1)
	_, err = os.Stat(logFile2)
	assert.NoError(t, err, "log_2 should not have been deleted")
	assert.FileExistsf(t, logFile2, "Log File: %s should exist", logFile2)
	_, err = os.Stat(logFile3)
	assert.NoError(t, err, "log_3 should not have been deleted")
	assert.FileExistsf(t, logFile3, "Log File: %s should exist", logFile3)
}
func TestExtensionLogger_Error(t *testing.T) {
	// Define the log directory
	var (
		logDir  = "/tmp/logs"
		fakeEnv = &handlerenv.HandlerEnvironment{}
	)

	err := createDirectories(logDir)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDir)

	fakeEnv.LogFolder = logDir

	// Create an ExtensionLogger with the fake logger
	logger, err := NewExtensionLogger(fakeEnv)
	require.NoError(t, err, "Failed to create logger")
	defer logger.Close()

	// Log an error message
	logger.Error("Found an error", slog.Any("error", fmt.Errorf("test error")))

	// Verify that the log file was created
	logFile := logger.(*ExtensionLogger).file
	assert.FileExists(t, logFile.Name(), "log file should have been created")

	// Read the log file
	logData, err := os.ReadFile(logFile.Name())
	assert.NoError(t, err)

	// Verify that the log message contains the error
	logOutput := string(logData)
	assert.Contains(t, logOutput, `event="Found an error`, "log message should contain the error message")
	assert.Contains(t, logOutput, `error.message="test error"`, "log message should contain the error message")
	assert.Contains(t, logOutput, "error.type=*errors.errorString", "log message should contain the error type")
	assert.Contains(t, logOutput, "error.stacktrace=", "log message should contain the error stack trace")
}

func TestLogger_LogsAppearInCorrectOrder(t *testing.T) {
	// Define the log directory
	var (
		logDir = "/tmp/logs"
	)

	// Create the log directory
	createDirectories(logDir)
	defer removeDirectories(logDir)

	// Create a logger
	logger, err := NewExtensionLoggerWithName(logDir, "log_%v_test")
	require.NoError(t, err, "Failed to create logger")
	defer logger.Close()

	// Log some messages with different levels and properties
	logs := []struct {
		Level string
		Msg   string
		Props []any
	}{
		{"Informational", "First message", []any{slog.Any("operation", "enable")}},
		{"Warning", "Second message", []any{slog.Any("operation", "enable")}},
		{"Error", "Third message", []any{slog.Any("operation", "enable")}},
		{"Critical", "Fourth message", []any{slog.Any("operation", "install")}},
	}

	// Convert the properties from []any to []slog.Attr
	getLogAttrs := func(log struct {
		Level string
		Msg   string
		Props []any
	}) []slog.Attr {
		attrs := make([]slog.Attr, len(log.Props))
		for i, p := range log.Props {
			attrs[i] = p.(slog.Attr)
		}
		return attrs
	}

	for _, log := range logs {
		switch log.Level {
		case "Informational":
			logger.Info(log.Msg, log.Props...)
		case "Warning":
			logger.Warn(log.Msg, log.Props...)
		case "Error":
			attrs := getLogAttrs(log)
			logger.Error(log.Msg, attrs...)
		case "Critical":
			attrs := getLogAttrs(log)
			logger.Critical(log.Msg, attrs...)
		}
	}

	// Get the log file name
	logFileName := logger.(*ExtensionLogger).file.Name()

	// Read the log file
	logData, err := os.ReadFile(logFileName)
	require.NoError(t, err, "Failed to read log file: %v")

	// Split the log output into lines
	logLines := strings.Split(string(logData), "\n")

	// Check that each log appears in the log output in the correct order
	for i, log := range logs {

		// Split the log line into parts
		l := logLines[i]
		// Check that the log entry contains the expected fields
		require.Contains(t, l, fmt.Sprintf("level=%s", log.Level), "Log entry does not contain the logged level")
		require.Contains(t, l, fmt.Sprintf("event=\"%s\"", log.Msg), "Log entry does not contain the logged message")

		// Check that the log entry contains the properties added with With()
		for _, prop := range log.Props {
			p := prop.(slog.Attr)
			propString := fmt.Sprintf("%v=%v", p.Key, p.Value)
			require.Contains(t, l, propString, "Log entry does not contain property '%v'", p.Key)
		}
	}
}

func createDirectories(dirs ...string) error {
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory: %v", err)
		}
	}
	return nil
}

func removeDirectories(dirs ...string) error {
	for _, dir := range dirs {
		err := os.RemoveAll(dir)
		if err != nil {
			return err
		}
	}
	return nil
}
