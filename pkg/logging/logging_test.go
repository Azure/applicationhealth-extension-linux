package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mb = 1024 * 1024 // 1 MB
)

func TestNew(t *testing.T) {
	// Test creating a logger with a handler environment
	var (
		logDirPath = "/tmp/logs"
		fileName   = "log_test"
		fakeEnv    = &handlerenv.HandlerEnvironment{}
	)
	err := createDirectories(logDirPath)
	require.NoError(t, err)
	defer removeDirectories(logDirPath)

	fakeEnv.LogFolder = logDirPath

	logger, err := NewSlogLogger(fakeEnv, fileName)
	require.NoError(t, err, "Failed to create logger")

	assert.NotNil(t, logger)
}

func TestNewWithName_Success(t *testing.T) {
	var (
		logDirPath = "/tmp/logs"
		fileName   = "log_test"
	)
	err := createDirectories(logDirPath)
	require.NoError(t, err)
	defer removeDirectories(logDirPath)

	logger, err := NewRotatingSlogLogger(logDirPath, fileName)
	require.NoError(t, err, "Failed to create logger")
	require.NotNil(t, logger, "Logger should not be nil")
}

func TestRotatingSlogLogger_NoDirExist(t *testing.T) {
	// Test creating a logger with a handler environment and custom log file name format
	var (
		logDirPath = "/tmp/logs"
		fileName   = "log_test"
		logger, _  = NewRotatingSlogLogger(logDirPath, fileName)
	)
	defer removeDirectories(logDirPath)
	require.NoDirExists(t, logDirPath, "Log directory should not have been created")
	logger.Info("Test log")
	require.FileExistsf(t, path.Join(logDirPath, fileName), "Log file should not have been created")
	require.DirExists(t, logDirPath, "Log directory should have been created")
}

func TestRotateLogFolder(t *testing.T) {
	var (
		logDirPath = "/tmp/logs"
		fileName   = "log_test"
	)
	err := createDirectories(logDirPath)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDirPath)

	rotatingWriter := lumberjack.Logger{
		Filename:   path.Join(logDirPath, fileName),
		MaxSize:    5, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}
	logger := createSlogLogger(&rotatingWriter)

	// Create some log files in the log folder
	logFile := filepath.Join(logDirPath, fileName)
	logger.Info("First log message")

	// Generate a large amount of data to write to the log files
	largeData := make([]byte, 2*mb) // 2 MB
	for i := range largeData {
		largeData[i] = 'A'
	}

	logger.Info("Adding large data to log file", slog.String("data", string(largeData)))
	logger.Info("Adding large data to log file", slog.String("data", string(largeData)))
	logger.Info("Adding large data to log file", slog.String("data", string(largeData)))

	// err = os.WriteFile(logFile1, largeData, 0644)
	// assert.NoError(t, err)

	// Verify that the current log file size is smaller than 5MB
	fileInfo, err := os.Stat(logFile)
	// Verify that only the newest log file remains
	assert.NoError(t, err, "Original log file should still exist")
	assert.FileExistsf(t, logFile, "Log File: %s should have been deleted by file rotation", logFile)
	// Verify that the current log file size is smaller than 5MB
	assert.Less(t, fileInfo.Size(), int64(5*mb), "Current log file size should be smaller than 5MB")

	// Verify that a .gz file was created
	files, err := os.ReadDir(logDirPath)
	require.NoError(t, err, "Failed to read log directory")
	var gzFileFound, timestampedFileFound bool
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".gz") {
			gzFileFound = true
		}
		if strings.HasPrefix(file.Name(), fileName) && file.Name() != fileName {
			timestampedFileFound = true
		}
	}

	assert.True(t, gzFileFound, "A .gz file should have been created")
	assert.True(t, timestampedFileFound, "A timestamped log file should have been created")

}
func TestExtensionLogger_Error(t *testing.T) {
	var (
		logDirPath = "/tmp/logs"
		fileName   = "log_test"
		fakeEnv    = &handlerenv.HandlerEnvironment{}
	)
	err := createDirectories(logDirPath)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDirPath)

	fakeEnv.LogFolder = logDirPath

	// Create an ExtensionLogger with the fake logger
	logger, err := NewSlogLogger(fakeEnv, fileName)
	require.NoError(t, err, "Failed to create logger")

	// Log an error message
	logger.Error("Found an error", slog.Any("error", fmt.Errorf("test error")))

	// Verify that the log file was created
	logFile := path.Join(logDirPath, fileName)
	assert.FileExists(t, logFile, "log file should have been created")
	logger.Info("Log file created", slog.String("path", logFile))

	// Read the log file
	logData, err := os.ReadFile(logFile)
	assert.NoError(t, err)

	// Verify that the log message contains the error
	logOutput := string(logData)
	assert.Contains(t, logOutput, `event="Found an error`, "log message should contain the error message")
	assert.Contains(t, logOutput, `error.message="test error"`, "log message should contain the error message")
	assert.Contains(t, logOutput, "error.type=*errors.errorString", "log message should contain the error type")
	assert.Contains(t, logOutput, "error.stacktrace=", "log message should contain the error stack trace")
}

func TestLogger_LogsAppearInCorrectOrder(t *testing.T) {
	var (
		logDirPath = "/tmp/logs"
		fileName   = "log_test"
	)
	err := createDirectories(logDirPath)
	require.NoError(t, err, "Failed to create log directory: %v")
	defer removeDirectories(logDirPath)

	// Create a logger
	logger, err := NewRotatingSlogLogger(logDirPath, fileName)
	require.NoError(t, err, "Failed to create logger")

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
			logger.LogAttrs(context.Background(), slog.LevelError, log.Msg, attrs...)
		case "Critical":
			attrs := getLogAttrs(log)
			logger.LogAttrs(context.Background(), LevelCritical, log.Msg, attrs...)
		}
	}

	// Verify that the log file was created
	logFile := path.Join(logDirPath, fileName)
	assert.FileExists(t, logFile, "log file should have been created")
	logger.Info("Log file created", slog.String("path", logFile))

	// Read the log file
	logData, err := os.ReadFile(logFile)
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
