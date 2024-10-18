package logging

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	slogformatter "github.com/samber/slog-formatter"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const (
	LevelCritical  = slog.Level(10)
	LevelLogAlways = slog.Level(1)
)

const (
	thirtyMB            = 30 * 1024 * 1024 // 31,457,280 bytes
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

// NewSlogLogger creates a new slog.Logger instance using the provided HandlerEnvironment.
// If the HandlerEnvironment is nil or the LogFolder within the HandlerEnvironment is empty,
// a standard output logger will be used.
// It returns a slog.Logger instance.
func NewSlogLogger(he *handlerenv.HandlerEnvironment, logFileName string) (*slog.Logger, error) {
	if he == nil || he.LogFolder == "" {
		// If log folder is not provided, use standard output
		return NewRotatingSlogLogger("", "")
	}

	return NewRotatingSlogLogger(he.LogFolder, logFileName)
}

// NewRotatingSlogLogger creates a new slog.Logger with the given log folder and log file format.
// If the log folder is not provided, it uses standard output as the logger.
// Supports custom log file format, with default format "log_%v".
// Supports cycling of logs to prevent filling up the disk.
// If valid LogFolder is provided, it will create and write logs to the specified folder.
func NewRotatingSlogLogger(logFolder string, logFileName string) (*slog.Logger, error) {
	if logFolder == "" {
		return createSlogLogger(nil), nil
	}

	if logFileName == "" {
		logFileName = "ApplicationHealthExtension.log"
	}

	rotatingWriter := lumberjack.Logger{
		Filename:   path.Join(logFolder, logFileName),
		MaxSize:    5, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}

	output := io.MultiWriter(os.Stdout, &rotatingWriter)
	return createSlogLogger(output), nil
}

func createSlogLogger(output io.Writer) *slog.Logger {
	if output == nil {
		output = os.Stdout
	}

	timeFormatter := slogformatter.TimeFormatter(time.RFC3339Nano, time.UTC)
	errorFormatter := slogformatter.ErrorFormatter("error")

	opts := slog.HandlerOptions{
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
	}

	return slog.New(slogformatter.NewFormatterHandler(timeFormatter, errorFormatter)(
		NewExtensionSlogHandler(output, &ExtensionHandlerOptions{SlogOpts: opts}),
	))
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

// NopLogger is a logger implementation that discards all log messages.
// It Implements the Logger interface from the Azure-extension-platform package.
type NopLogger struct {
}

func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

func (l NopLogger) Info(format string, v ...interface{}) {
	// No-op
}

func (l NopLogger) Warn(format string, v ...interface{}) {
	// No-op
}

func (l NopLogger) Error(format string, v ...interface{}) {
	// No-op
}

func (l NopLogger) WarnFromStream(prefix string, streamReader io.Reader) {
	// No-op
}

func (l NopLogger) InfoFromStream(prefix string, streamReader io.Reader) {
	// No-op
}

func (l NopLogger) ErrorFromStream(prefix string, streamReader io.Reader) {
	// No-op
}

func (l NopLogger) Close() {
	// No-op
}
