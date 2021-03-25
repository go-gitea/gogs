// Copyright 2020 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// MultiChannelledLogger is default logger in the Gitea application.
// it can contain several providers and log message into all providers.
type MultiChannelledLogger struct {
	LevelLoggerLogger
	*MultiChannelledLog
	bufferLength int64
}

// newLogger initializes and returns a new logger.
func newLogger(name string, buffer int64) *MultiChannelledLogger {
	l := &MultiChannelledLogger{
		MultiChannelledLog: NewMultiChannelledLog(name, buffer),
		bufferLength:       buffer,
	}
	l.LevelLogger = l
	return l
}

// SetLogger sets new logger instance with given logger provider and config.
func (l *MultiChannelledLogger) SetLogger(name, provider, config string) error {
	eventLogger, err := NewChannelledLog(name, provider, config, l.bufferLength)
	if err != nil {
		return fmt.Errorf("Failed to create sublogger (%s): %v", name, err)
	}

	l.MultiChannelledLog.DelLogger(name)

	err = l.MultiChannelledLog.AddLogger(eventLogger)
	if err != nil {
		if IsErrDuplicateName(err) {
			return fmt.Errorf("Duplicate named sublogger %s %v", name, l.MultiChannelledLog.GetEventLoggerNames())
		}
		return fmt.Errorf("Failed to add sublogger (%s): %v", name, err)
	}

	return nil
}

// DelLogger deletes a sublogger from this logger.
func (l *MultiChannelledLogger) DelLogger(name string) (bool, error) {
	return l.MultiChannelledLog.DelLogger(name), nil
}

// Log msg at the provided level with the provided caller defined by skip (0 being the function that calls this function)
func (l *MultiChannelledLogger) Log(skip int, level Level, format string, v ...interface{}) error {
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
		msg = ColorSprintf(format, v...)
	}
	stack := ""
	if l.GetStacktraceLevel() <= level {
		stack = Stack(skip + 1)
	}
	return l.SendLog(level, caller, strings.TrimPrefix(filename, prefix), line, msg, stack)
}

// SendLog sends a log event at the provided level with the information given
func (l *MultiChannelledLogger) SendLog(level Level, caller, filename string, line int, msg, stack string) error {
	if l.GetLevel() > level {
		return nil
	}
	event := &Event{
		level:      level,
		caller:     caller,
		filename:   filename,
		line:       line,
		msg:        msg,
		time:       time.Now(),
		stacktrace: stack,
	}
	l.LogEvent(event)
	return nil
}
