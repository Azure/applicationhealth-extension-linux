package logging

import (
	"io"

	"github.com/go-kit/log"
)

// NopLogger is a logger implementation that discards all log messages.
// It Implements the Logger interface from the Azure-extension-platform package.
type NopLogger struct {
	log.Logger
}

func NewNopLogger() *NopLogger {
	return &NopLogger{
		Logger: log.NewNopLogger(),
	}
}

func (l NopLogger) Info(format string, v ...interface{}) {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}

func (l NopLogger) Warn(format string, v ...interface{}) {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}

func (l NopLogger) Error(format string, v ...interface{}) {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}

func (l NopLogger) ErrorFromStream(prefix string, streamReader io.Reader) {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}

func (l NopLogger) WarnFromStream(prefix string, streamReader io.Reader) {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}

func (l NopLogger) InfoFromStream(prefix string, streamReader io.Reader) {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}

func (l NopLogger) Close() {
	err := l.Log()
	if err != nil {
		panic(err)
	}
}
