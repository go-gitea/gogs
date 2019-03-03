// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// Logger is default logger in the Gitea application.
// it can contain several providers and log message into all providers.
type Logger struct {
	EventLogger
	bufferLength int64
}

// newLogger initializes and returns a new logger.
func newLogger(name string, buffer int64) *Logger {
	l := &Logger{
		EventLogger:  CreateMultiChannelledLog(name, buffer),
		bufferLength: buffer,
	}
	return l
}

// SetLogger sets new logger instance with given logger provider and config.
func (l *Logger) SetLogger(name, provider, config string) error {
	m, ok := l.EventLogger.(*MultiChannelledLog)
	if !ok {
		return fmt.Errorf("Unable to add to this logger")
	}
	eventLogger, err := CreateChannelledLog(name, provider, config, l.bufferLength)
	if err != nil {
		return fmt.Errorf("Failed to create sublogger (%s): %v", name, err)
	}

	m.DelLogger(name)

	err = m.AddLogger(eventLogger)
	if err != nil {
		if IsErrDuplicateName(err) {

			return fmt.Errorf("Duplicate named sublogger %s %v", name, m.GetEventLoggerNames())
		}
		return fmt.Errorf("Failed to add sublogger (%s): %v", name, err)
	}

	return nil
}

// DelLogger deletes a sublogger from this logger.
func (l *Logger) DelLogger(name string) (bool, error) {
	m, ok := l.EventLogger.(*MultiChannelledLog)
	if !ok {
		return false, fmt.Errorf("Unable to delete from this logger")
	}
	return m.DelLogger(name), nil
}

// GetLogger gets sublogger from this logger and wraps it as Logger.
// NB you should not close this logger
func (l *Logger) GetLogger(name string) *Logger {
	m, ok := l.EventLogger.(*MultiChannelledLog)
	if !ok {
		return nil
	}
	eventLogger := m.GetEventLogger(name)
	if eventLogger == nil {
		return nil
	}
	return &Logger{
		EventLogger:  eventLogger,
		bufferLength: l.bufferLength,
	}
}

// Log msg at the provided level with the provided caller defined by skip (0 being the function that calls this function)
func (l *Logger) Log(skip int, level Level, format string, v ...interface{}) error {
	if l.GetLevel() > level {
		return nil
	}
	caller := "?()"
	pc, filename, line, ok := runtime.Caller(skip + 1)
	if ok {
		// Get caller function name.
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			caller = fn.Name() + "()"
		}
	}
	msg := format
	if len(v) > 0 {
		msg = fmt.Sprintf(format, v...)
	}
	return l.SendLog(level, caller, strings.TrimPrefix(filename, prefix), line, msg)
}

// SendLog sends a log event at the provided level with the information given
func (l *Logger) SendLog(level Level, caller, filename string, line int, msg string) error {
	if l.GetLevel() > level {
		return nil
	}
	event := &Event{
		level:    level,
		caller:   caller,
		filename: filename,
		line:     line,
		msg:      msg,
		time:     time.Now(),
	}
	l.LogEvent(event)
	return nil
}

// Trace records trace log
func (l *Logger) Trace(format string, v ...interface{}) {
	l.Log(1, TRACE, format, v...)
}

// Debug records debug log
func (l *Logger) Debug(format string, v ...interface{}) {
	l.Log(1, DEBUG, format, v...)

}

// Info records information log
func (l *Logger) Info(format string, v ...interface{}) {
	l.Log(1, INFO, format, v...)
}

// Warn records warning log
func (l *Logger) Warn(format string, v ...interface{}) {
	l.Log(1, WARN, format, v...)
}

// Error records error log
func (l *Logger) Error(skip int, format string, v ...interface{}) {
	l.Log(skip+1, ERROR, format, v...)
}

// Critical records critical log
func (l *Logger) Critical(skip int, format string, v ...interface{}) {
	l.Log(skip+1, CRITICAL, format, v...)
}

// Fatal records error log and exit the process
func (l *Logger) Fatal(skip int, format string, v ...interface{}) {
	l.Log(skip+1, FATAL, format, v...)
	l.Close()
	os.Exit(1)
}
