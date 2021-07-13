// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"io"
	"log"
)

// LogLevel defines a log level
type LogLevel int

// enumerate all LogLevels
const (
	// !nashtsai! following level also match syslog.Priority value
	LOG_DEBUG LogLevel = iota
	LOG_INFO
	LOG_WARNING
	LOG_ERR
	LOG_OFF
	LOG_UNKNOWN
)

// default log options
const (
	DEFAULT_LOG_PREFIX = "[xorm]"
	DEFAULT_LOG_FLAG   = log.Ldate | log.Lmicroseconds
	DEFAULT_LOG_LEVEL  = LOG_DEBUG
)

// Logger is a logger interface
type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Warn(v ...interface{})
	Warnf(format string, v ...interface{})

	Level() LogLevel
	SetLevel(l LogLevel)

	ShowSQL(show ...bool)
	IsShowSQL() bool
}

var _ Logger = DiscardLogger{}

// DiscardLogger don't log implementation for ILogger
type DiscardLogger struct{}

// Debug empty implementation
func (DiscardLogger) Debug(v ...interface{}) {}

// Debugf empty implementation
func (DiscardLogger) Debugf(format string, v ...interface{}) {}

// Error empty implementation
func (DiscardLogger) Error(v ...interface{}) {}

// Errorf empty implementation
func (DiscardLogger) Errorf(format string, v ...interface{}) {}

// Info empty implementation
func (DiscardLogger) Info(v ...interface{}) {}

// Infof empty implementation
func (DiscardLogger) Infof(format string, v ...interface{}) {}

// Warn empty implementation
func (DiscardLogger) Warn(v ...interface{}) {}

// Warnf empty implementation
func (DiscardLogger) Warnf(format string, v ...interface{}) {}

// Level empty implementation
func (DiscardLogger) Level() LogLevel {
	return LOG_UNKNOWN
}

// SetLevel empty implementation
func (DiscardLogger) SetLevel(l LogLevel) {}

// ShowSQL empty implementation
func (DiscardLogger) ShowSQL(show ...bool) {}

// IsShowSQL empty implementation
func (DiscardLogger) IsShowSQL() bool {
	return false
}

// SimpleLogger is the default implment of ILogger
type SimpleLogger struct {
	DEBUG   *log.Logger
	ERR     *log.Logger
	INFO    *log.Logger
	WARN    *log.Logger
	level   LogLevel
	showSQL bool
}

var _ Logger = &SimpleLogger{}

// NewSimpleLogger use a special io.Writer as logger output
func NewSimpleLogger(out io.Writer) *SimpleLogger {
	return NewSimpleLogger2(out, DEFAULT_LOG_PREFIX, DEFAULT_LOG_FLAG)
}

// NewSimpleLogger2 let you customrize your logger prefix and flag
func NewSimpleLogger2(out io.Writer, prefix string, flag int) *SimpleLogger {
	return NewSimpleLogger3(out, prefix, flag, DEFAULT_LOG_LEVEL)
}

// NewSimpleLogger3 let you customrize your logger prefix and flag and logLevel
func NewSimpleLogger3(out io.Writer, prefix string, flag int, l LogLevel) *SimpleLogger {
	return &SimpleLogger{
		DEBUG: log.New(out, fmt.Sprintf("%s [debug] ", prefix), flag),
		ERR:   log.New(out, fmt.Sprintf("%s [error] ", prefix), flag),
		INFO:  log.New(out, fmt.Sprintf("%s [info]  ", prefix), flag),
		WARN:  log.New(out, fmt.Sprintf("%s [warn]  ", prefix), flag),
		level: l,
	}
}

// Error implement ILogger
func (s *SimpleLogger) Error(v ...interface{}) {
	if s.level <= LOG_ERR {
		s.ERR.Output(2, fmt.Sprintln(v...))
	}
}

// Errorf implement ILogger
func (s *SimpleLogger) Errorf(format string, v ...interface{}) {
	if s.level <= LOG_ERR {
		s.ERR.Output(2, fmt.Sprintf(format, v...))
	}
}

// Debug implement ILogger
func (s *SimpleLogger) Debug(v ...interface{}) {
	if s.level <= LOG_DEBUG {
		s.DEBUG.Output(2, fmt.Sprintln(v...))
	}
}

// Debugf implement ILogger
func (s *SimpleLogger) Debugf(format string, v ...interface{}) {
	if s.level <= LOG_DEBUG {
		s.DEBUG.Output(2, fmt.Sprintf(format, v...))
	}
}

// Info implement ILogger
func (s *SimpleLogger) Info(v ...interface{}) {
	if s.level <= LOG_INFO {
		s.INFO.Output(2, fmt.Sprintln(v...))
	}
}

// Infof implement ILogger
func (s *SimpleLogger) Infof(format string, v ...interface{}) {
	if s.level <= LOG_INFO {
		s.INFO.Output(2, fmt.Sprintf(format, v...))
	}
}

// Warn implement ILogger
func (s *SimpleLogger) Warn(v ...interface{}) {
	if s.level <= LOG_WARNING {
		s.WARN.Output(2, fmt.Sprintln(v...))
	}
}

// Warnf implement ILogger
func (s *SimpleLogger) Warnf(format string, v ...interface{}) {
	if s.level <= LOG_WARNING {
		s.WARN.Output(2, fmt.Sprintf(format, v...))
	}
}

// Level implement ILogger
func (s *SimpleLogger) Level() LogLevel {
	return s.level
}

// SetLevel implement ILogger
func (s *SimpleLogger) SetLevel(l LogLevel) {
	s.level = l
}

// ShowSQL implement ILogger
func (s *SimpleLogger) ShowSQL(show ...bool) {
	if len(show) == 0 {
		s.showSQL = true
		return
	}
	s.showSQL = show[0]
}

// IsShowSQL implement ILogger
func (s *SimpleLogger) IsShowSQL() bool {
	return s.showSQL
}
