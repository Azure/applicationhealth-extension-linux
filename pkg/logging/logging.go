package logging

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	slogformatter "github.com/samber/slog-formatter"
)

const (
	LevelCritical  = slog.Level(10)
	LevelLogAlways = slog.Level(1)
)

const (
	thirtyMB            = 30 * 1024 * 1034 // 31,457,280 bytes
	fortyMB             = 40 * 1024 * 1024 // 41,943,040 bytes
	logDirThresholdLow  = thirtyMB
	logDirThresholdHigh = fortyMB
)

var (
	LevelNames = map[slog.Leveler]string{
		LevelCritical:   "Critical",
		LevelLogAlways:  "LogAlways",
		slog.LevelError: "Error",
		slog.LevelWarn:  "Warning",
		slog.LevelInfo:  "Informational",
		slog.LevelDebug: "Debug",
	}
)

// Logger Interface for Guest Health Platform/Application Health Extension.
// This interface will allow use to use different logging libraries if needed in future.
type Logger interface {
	Log(ctx context.Context, level slog.Level, msg string, args ...any)
	LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, attrs ...slog.Attr)
	Critical(msg string, attrs ...slog.Attr)
	Debug(msg string, args ...any)
	Close() error
	With(args ...any)
}

// ExtensionLogger is our concrete implementation of the Logger interface.
// It functions as an adapter/wrapper to the slog.Logger, which is a wrapper around the standard log.Logger
type ExtensionLogger struct {
	*slog.Logger
	file *os.File
}

// NewExtensionLogger creates a new logging instance using the provided HandlerEnvironment.
// If the HandlerEnvironment is nil or the LogFolder within the HandlerEnvironment is empty,
// a standard output logger will be used.
// It returns a Logger instance.
func NewExtensionLogger(he *handlerenv.HandlerEnvironment) Logger {
	if he == nil || he.HandlerEnvironment.LogFolder == "" {
		// Standard output logger will be used
		return NewExtensionLoggerWithName("", "")
	}

	return NewExtensionLoggerWithName(he.HandlerEnvironment.LogFolder, "")
}

// NewExtensionLoggerWithName creates a new logger with the given log folder and log file format.
// If the log folder is not provided, it uses standard output as the logger.
// Supports custom log file format, with default format "log_%v".
// Supports cycling of logs to prevent filling up the disk.
// If valid LogFolder is provided, it will create and write logs to the specified folder.
func NewExtensionLoggerWithName(logFolder string, logFileFormat string) Logger {
	if logFolder == "" {
		// If handler environment is not provided, use standard output
		return newStandardOutput()
	}

	if logFileFormat == "" {
		logFileFormat = "log_%v"
	}

	// Rotate log folder to prevent filling up the disk
	err := rotateLogFolder(logFolder, logFileFormat)
	if err != nil {
		return newStandardOutput()
	}

	fileName := fmt.Sprintf(logFileFormat, strconv.FormatInt(time.Now().UTC().Unix(), 10))
	filePath := path.Join(logFolder, fileName)
	writer, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return newStandardOutput()
	}
	return newMultiLogger(writer)
}

// Error logs an error. Format is the same as fmt.Print
func (l *ExtensionLogger) Error(msg string, attrs ...slog.Attr) {
	l.Logger.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

func (l *ExtensionLogger) Critical(msg string, attrs ...slog.Attr) {
	l.Logger.LogAttrs(context.Background(), LevelCritical, msg, attrs...)
}

// Close closes the file
func (l *ExtensionLogger) Close() error {
	if l.file == nil {
		return fmt.Errorf("file must be non-nil")
	}
	return l.file.Close()
}

func (l *ExtensionLogger) With(args ...any) {
	l.Logger = l.Logger.With(args...)
}

func newStandardOutput() *ExtensionLogger {
	timeFormatter := slogformatter.TimeFormatter(time.RFC3339Nano, time.UTC)
	errorFormatter := slogformatter.ErrorFormatter("error")
	return &ExtensionLogger{
		Logger: slog.New(slogformatter.NewFormatterHandler(timeFormatter, errorFormatter)(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.LevelKey {
							level := a.Value.Any().(slog.Level)
							levelLabel, exist := LevelNames[level]
							if !exist {
								levelLabel = level.String()
							}
							a.Value = slog.StringValue(levelLabel)
						}
						return a
					},
				}))).With(slog.Int("pid", os.Getpid())),
		file: nil,
	}
}

func newMultiLogger(writer *os.File) *ExtensionLogger {
	timeFormatter := slogformatter.TimeFormatter(time.RFC3339Nano, time.UTC)
	errorFormatter := slogformatter.ErrorFormatter("error")
	return &ExtensionLogger{
		Logger: slog.New(slogformatter.NewFormatterHandler(timeFormatter, errorFormatter)(
			slog.NewTextHandler(
				io.MultiWriter(os.Stdout, writer),
				&slog.HandlerOptions{
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.LevelKey {
							level := a.Value.Any().(slog.Level)
							levelLabel, exist := LevelNames[level]
							if !exist {
								levelLabel = level.String()
							}
							a.Value = slog.StringValue(levelLabel)
						}
						return a
					},
				}))).With(slog.Int("pid", os.Getpid())),
		file: writer,
	}
}

// Function to get directory size
func getDirSize(dirPath string) (size int64, err error) {
	err = filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		}

		return nil
	})

	if err != nil {
		err = fmt.Errorf("unable to compute directory size, error: %v", err)
	}
	return
}

// Function to rotate log files present in logFolder to avoid filling customer disk space
// File name matching is done on file name pattern provided before '%'
func rotateLogFolder(logFolder string, logFileFormat string) (err error) {
	size, err := getDirSize(logFolder)
	if err != nil {
		return
	}

	// If directory size is still under high threshold value, nothing to do
	if size < logDirThresholdHigh {
		return
	}

	// Get all log files in logFolder
	// Files are already sorted according to filenames
	// Log file names contains unix timestamp as suffix, Thus we have files sorted according to age as well
	var dirEntries []fs.DirEntry

	dirEntries, err = os.ReadDir(logFolder)
	if err != nil {
		err = fmt.Errorf("unable to read log folder, error: %v", err)
		return
	}

	// Converting to FileInfo to sort according to time
	dirEntriesInfo := make([]fs.FileInfo, len(dirEntries))
	for i, entry := range dirEntries {
		dirEntriesInfo[i], err = entry.Info()
		if err != nil {
			err = fmt.Errorf("unable to get file info, error: %v", err)
			return
		}
	}

	// Sort directory entries according to time (oldest to newest)
	sort.Slice(dirEntriesInfo, func(idx1, idx2 int) bool {
		return dirEntriesInfo[idx1].ModTime().Before(dirEntriesInfo[idx2].ModTime())
	})

	// Get log file name prefix
	logFilePrefix := strings.Split(logFileFormat, "%")

	for _, file := range dirEntriesInfo {
		// Once directory size goes below lower threshold limit, stop deletion
		if size < logDirThresholdLow {
			break
		}

		// Skip directories
		if file.IsDir() {
			continue
		}

		// log file names are prefixed according to logFileFormat specified
		if !strings.HasPrefix(file.Name(), logFilePrefix[0]) {
			continue
		}

		// Delete the file
		err = os.Remove(filepath.Join(logFolder, file.Name()))
		if err != nil {
			err = fmt.Errorf("unable to delete log files, error: %v", err)
			return
		}

		// Subtract file size from total directory size
		size = size - file.Size()
	}
	return
}
