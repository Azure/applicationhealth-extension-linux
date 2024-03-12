package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// Creating an Adaptor for the PlatformLogger interface to use our own Logger
type LoggerAdaptor struct {
	logger Logger
	noop   bool
}

// Given an already configured logger, this function returns a new logger with the same configuration
func NewAdaptor(lg Logger) (*LoggerAdaptor, error) {
	if s, ok := lg.(*ExtensionLogger); ok {
		return &LoggerAdaptor{
			logger: s,
		}, nil
	} else {
		return nil, fmt.Errorf("Logger does not satisfy the ExtensionLogger interface")
	}
}

func (l *LoggerAdaptor) SetNoop(noop bool) {
	l.noop = noop
}

// Extension-platform may use this similar to fmt.Sprintf
func (l *LoggerAdaptor) Error(format string, v ...interface{}) {
	if l.noop {
		return
	}
	if len(v) == 0 {
		l.logger.Error(format)
	} else {
		l.logger.Error(fmt.Sprintf(format, v...), slog.Any("error", fmt.Errorf(fmt.Sprintf(format, v...))))
	}
}

// Extension-platform may use this similar to fmt.Sprintf
func (l *LoggerAdaptor) Warn(format string, v ...interface{}) {
	if l.noop {
		return
	}
	if len(v) == 0 {
		l.logger.Warn(format)
	} else {
		l.logger.Warn(fmt.Sprintf(format, v...))
	}
}

// Extension-platform may use this similar to fmt.Sprintf
func (l *LoggerAdaptor) Info(format string, v ...interface{}) {
	if l.noop {
		return
	}

	if len(v) == 0 {
		l.logger.Info(format)
	} else {
		l.logger.Info(fmt.Sprintf(format, v...))
	}
}

func (l *LoggerAdaptor) Close() {
	l.logger.Close()
}

func (l *LoggerAdaptor) ErrorFromStream(prefix string, streamReader io.Reader) {}

func (l *LoggerAdaptor) WarnFromStream(prefix string, streamReader io.Reader) {}

func (l *LoggerAdaptor) InfoFromStream(prefix string, streamReader io.Reader) {}
