package logging

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/applicationhealth-extension-linux/internal/handlerenv"
	"github.com/Azure/applicationhealth-extension-linux/pkg/utils"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/log"
)

const (
	logError        = "error"
	logLevelWarning = "Warning "
	logLevelInfo    = "Info "
	logEvent        = "event"
	logMessage      = "message"
)

const (
	thirtyMB            = 30 * 1024 * 1034 // 31,457,280 bytes
	fortyMB             = 40 * 1024 * 1024 // 41,943,040 bytes
	logDirThresholdLow  = thirtyMB
	logDirThresholdHigh = fortyMB
)

// ExtensionLogger for all the extension related events
type ExtensionLogger struct {
	logger log.Logger // make this private again
	file   *os.File
}

// New creates a new logging instance. If the handlerEnvironment is nil, we'll use a
// standard output logger
func New(he *handlerenv.HandlerEnvironment) *ExtensionLogger {
	return NewWithName(he, "")
}

// Allows the caller to specify their own name for the file
// Supports cycling of logs to prevent filling up the disk
func NewWithName(he *handlerenv.HandlerEnvironment, logFileFormat string) *ExtensionLogger {
	if he == nil {
		// If handler environment is not provided, use standard output
		return newStandardOutput()
	}

	if logFileFormat == "" {
		logFileFormat = "log_%v"
	}

	// Rotate log folder to prevent filling up the disk
	err := rotateLogFolder(he.HandlerEnvironment.LogFolder, logFileFormat)
	if err != nil {
		return newStandardOutput()
	}

	fileName := fmt.Sprintf(logFileFormat, strconv.FormatInt(time.Now().UTC().Unix(), 10))
	filePath := path.Join(he.HandlerEnvironment.LogFolder, fileName)
	fileWriter, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return newStandardOutput()
	}
	// logger will log on both standard output and file
	logger := log.NewSyncLogger(log.NewLogfmtLogger(io.MultiWriter(os.Stdout, fileWriter)))
	return &ExtensionLogger{
		logger: logger,
		file:   fileWriter,
	}
}

func GetCallStack() string {
	return string(debug.Stack())
}

// Function to add Time Context to the logger
func (lg *ExtensionLogger) WithTimestamp() {
	lg.logger = log.With(lg.logger, "time", log.DefaultTimestampUTC)
}

func (lg *ExtensionLogger) WithProcessID() {
	lg.logger = log.With(lg.logger, "pid", utils.GetCurrentProcessID())
}

func (lg *ExtensionLogger) WithSeqNum(seqNum string) {
	lg.logger = log.With(lg.logger, "seq", seqNum)
}

func (lg *ExtensionLogger) WithOperation(operation string) {
	lg.logger = log.With(lg.logger, "operation", operation)
}

// Function to add Version Context to the logger
func (lg *ExtensionLogger) WithVersion(versionString string) {
	lg.logger = log.With(lg.logger, "version", versionString)
}

func (lg *ExtensionLogger) With(key string, value string) {
	lg.logger = log.With(lg.logger, key, value)
}

func (lg *ExtensionLogger) Event(event string) {
	level.Info(lg.logger).Log(logEvent, event)
}

// Function to log an event with optional error details
func (lg ExtensionLogger) EventError(event string, errorDetails ...interface{}) {
	if len(errorDetails) == 0 {
		level.Error(lg.logger).Log(logEvent, event)
		return
	}
	switch v := errorDetails[0].(type) {
	case error:
		level.Error(lg.logger).Log(logEvent, fmt.Sprintf("%s, Details: %v", event, v.Error()))
	case string:
		level.Error(lg.logger).Log(logEvent, fmt.Sprintf("%s, Details: %s", event, v))
	default:
		level.Error(lg.logger).Log(logEvent, fmt.Sprintf("%s, Details: %v", event, v))
	}
}

func (lg ExtensionLogger) CustomLog(keyvals ...interface{}) {
	lg.logger.Log(keyvals...)
}

// Close closes the file
func (logger *ExtensionLogger) Close() {
	if logger.file != nil {
		logger.file.Close()
	}
}

func newStandardOutput() *ExtensionLogger {
	return &ExtensionLogger{
		logger: log.NewSyncLogger(log.NewLogfmtLogger(os.Stdout)),
		file:   nil,
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

		return err
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
	var dirEntries []fs.FileInfo

	dirEntries, err = ioutil.ReadDir(logFolder)
	if err != nil {
		err = fmt.Errorf("unable to read log folder, error: %v", err)
		return
	}

	// Sort directory entries according to time (oldest to newest)
	sort.Slice(dirEntries, func(idx1, idx2 int) bool {
		return dirEntries[idx1].ModTime().Before(dirEntries[idx2].ModTime())
	})

	// Get log file name prefix
	logFilePrefix := strings.Split(logFileFormat, "%")

	for _, file := range dirEntries {
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
