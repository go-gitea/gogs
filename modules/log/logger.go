// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import "os"

// Logger is the basic interface for logging
type Logger interface {
	LevelLogger
	Trace(format string, v ...interface{})
	IsTrace() bool
	Debug(format string, v ...interface{})
	IsDebug() bool
	Info(format string, v ...interface{})
	IsInfo() bool
	Warn(format string, v ...interface{})
	IsWarn() bool
	Error(format string, v ...interface{})
	ErrorWithSkip(skip int, format string, v ...interface{})
	IsError() bool
	Critical(format string, v ...interface{})
	CriticalWithSkip(skip int, format string, v ...interface{})
	IsCritical() bool
	Fatal(format string, v ...interface{})
	FatalWithSkip(skip int, format string, v ...interface{})
	IsFatal() bool
}

// LevelLogger is the simplest logging interface
type LevelLogger interface {
	Flush()
	Close()
	GetLevel() Level
	Log(skip int, level Level, format string, v ...interface{}) error
}

// SettableLogger is the interface of loggers which have subloggers
type SettableLogger interface {
	SetLogger(name, provider, config string) error
	DelLogger(name string) (bool, error)
}

// StacktraceLogger is a logger that can log stacktraces
type StacktraceLogger interface {
	GetStacktraceLevel() Level
}

// LevelLoggerLogger wraps a LevelLogger as a Logger
type LevelLoggerLogger struct {
	LevelLogger
}

// Trace records trace log
func (l *LevelLoggerLogger) Trace(format string, v ...interface{}) {
	l.Log(1, TRACE, format, v...)
}

// IsTrace returns true if the logger is TRACE
func (l *LevelLoggerLogger) IsTrace() bool {
	return l.GetLevel() <= TRACE
}

// Debug records debug log
func (l *LevelLoggerLogger) Debug(format string, v ...interface{}) {
	l.Log(1, DEBUG, format, v...)
}

// IsDebug returns true if the logger is DEBUG
func (l *LevelLoggerLogger) IsDebug() bool {
	return l.GetLevel() <= DEBUG
}

// Info records information log
func (l *LevelLoggerLogger) Info(format string, v ...interface{}) {
	l.Log(1, INFO, format, v...)
}

// IsInfo returns true if the logger is INFO
func (l *LevelLoggerLogger) IsInfo() bool {
	return l.GetLevel() <= INFO
}

// Warn records warning log
func (l *LevelLoggerLogger) Warn(format string, v ...interface{}) {
	l.Log(1, WARN, format, v...)
}

// IsWarn returns true if the logger is WARN
func (l *LevelLoggerLogger) IsWarn() bool {
	return l.GetLevel() <= WARN
}

// Error records error log
func (l *LevelLoggerLogger) Error(format string, v ...interface{}) {
	l.Log(1, ERROR, format, v...)
}

// ErrorWithSkip records error log from "skip" calls back from this function
func (l *LevelLoggerLogger) ErrorWithSkip(skip int, format string, v ...interface{}) {
	l.Log(skip+1, ERROR, format, v...)
}

// IsError returns true if the logger is ERROR
func (l *LevelLoggerLogger) IsError() bool {
	return l.GetLevel() <= ERROR
}

// Critical records critical log
func (l *LevelLoggerLogger) Critical(format string, v ...interface{}) {
	l.Log(1, CRITICAL, format, v...)
}

// CriticalWithSkip records critical log from "skip" calls back from this function
func (l *LevelLoggerLogger) CriticalWithSkip(skip int, format string, v ...interface{}) {
	l.Log(skip+1, CRITICAL, format, v...)
}

// IsCritical returns true if the logger is CRITICAL
func (l *LevelLoggerLogger) IsCritical() bool {
	return l.GetLevel() <= CRITICAL
}

// Fatal records fatal log and exit the process
func (l *LevelLoggerLogger) Fatal(format string, v ...interface{}) {
	l.Log(1, FATAL, format, v...)
	l.Close()
	os.Exit(1)
}

// FatalWithSkip records fatal log from "skip" calls back from this function and exits the process
func (l *LevelLoggerLogger) FatalWithSkip(skip int, format string, v ...interface{}) {
	l.Log(skip+1, FATAL, format, v...)
	l.Close()
	os.Exit(1)
}

// IsFatal returns true if the logger is FATAL
func (l *LevelLoggerLogger) IsFatal() bool {
	return l.GetLevel() <= FATAL
}
