// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

type byteArrayWriter []byte

func (b *byteArrayWriter) Write(p []byte) (int, error) {
	*b = append(*b, p...)
	return len(p), nil
}

// WriterLogger represent a basic logger for Gitea
type WriterLogger struct {
	out io.WriteCloser
	mu  sync.Mutex

	Level           Level  `json:"level"`
	StacktraceLevel Level  `json:"stacktraceLevel"`
	Flags           int    `json:"flags"`
	Prefix          string `json:"prefix"`
	Colorize        bool   `json:"colorize"`
	Expression      string `json:"expression"`
	regexp          *regexp.Regexp
}

// NewWriterLogger creates a new WriterLogger from the provided WriteCloser.
// Optionally the level can be changed at the same time.
func (logger *WriterLogger) NewWriterLogger(out io.WriteCloser, level ...Level) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.out = out
	switch logger.Flags {
	case 0:
		logger.Flags = LstdFlags
	case -1:
		logger.Flags = 0
	}
	if len(level) > 0 {
		logger.Level = level[0]
	}
	logger.createExpression()
}

func (logger *WriterLogger) createExpression() {
	if len(logger.Expression) > 0 {
		var err error
		logger.regexp, err = regexp.Compile(logger.Expression)
		if err != nil {
			logger.regexp = nil
		}
	}
}

// GetLevel returns the logging level for this logger
func (logger *WriterLogger) GetLevel() Level {
	return logger.Level
}

// GetStacktraceLevel returns the stacktrace logging level for this logger
func (logger *WriterLogger) GetStacktraceLevel() Level {
	return logger.StacktraceLevel
}

// Copy of cheap integer to fixed-width decimal to ascii from logger.
func itoa(buf *[]byte, i, wid int) {
	var logger [20]byte
	bp := len(logger) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		logger[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	logger[bp] = byte('0' + i)
	*buf = append(*buf, logger[bp:]...)
}

func (logger *WriterLogger) createMsg(buf *[]byte, event *Event) {
	*buf = append(*buf, logger.Prefix...)
	t := event.time
	if logger.Flags&(Ldate|Ltime|Lmicroseconds) != 0 {
		if logger.Colorize {
			*buf = append(*buf, fgCyanBytes...)
		}
		if logger.Flags&LUTC != 0 {
			t = t.UTC()
		}
		if logger.Flags&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if logger.Flags&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if logger.Flags&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
		if logger.Colorize {
			*buf = append(*buf, resetBytes...)
		}

	}
	if logger.Flags&(Lshortfile|Llongfile) != 0 {
		if logger.Colorize {
			*buf = append(*buf, fgGreenBytes...)
		}
		file := event.filename
		if logger.Flags&Lmedfile == Lmedfile {
			startIndex := len(file) - 20
			if startIndex > 0 {
				file = "..." + file[startIndex:]
			}
		} else if logger.Flags&Lshortfile != 0 {
			startIndex := strings.LastIndexByte(file, '/')
			if startIndex > 0 && startIndex < len(file) {
				file = file[startIndex+1:]
			}
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, event.line, -1)
		if logger.Flags&(Lfuncname|Lshortfuncname) != 0 {
			*buf = append(*buf, ':')
		} else {
			if logger.Colorize {
				*buf = append(*buf, resetBytes...)
			}
			*buf = append(*buf, ' ')
		}
	}
	if logger.Flags&(Lfuncname|Lshortfuncname) != 0 {
		if logger.Colorize {
			*buf = append(*buf, fgGreenBytes...)
		}
		funcname := event.caller
		if logger.Flags&Lshortfuncname != 0 {
			lastIndex := strings.LastIndexByte(funcname, '.')
			if lastIndex > 0 && len(funcname) > lastIndex+1 {
				funcname = funcname[lastIndex+1:]
			}
		}
		*buf = append(*buf, funcname...)
		if logger.Colorize {
			*buf = append(*buf, resetBytes...)
		}
		*buf = append(*buf, ' ')

	}
	if logger.Flags&(Llevel|Llevelinitial) != 0 {
		level := strings.ToUpper(event.level.String())
		if logger.Colorize {
			*buf = append(*buf, levelToColor[event.level]...)
		}
		*buf = append(*buf, '[')
		if logger.Flags&Llevelinitial != 0 {
			*buf = append(*buf, level[0])
		} else {
			*buf = append(*buf, level...)
		}
		*buf = append(*buf, ']')
		if logger.Colorize {
			*buf = append(*buf, resetBytes...)
		}
		*buf = append(*buf, ' ')
	}

	msg := []byte(event.msg)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	pawMode := allowColor
	if !logger.Colorize {
		pawMode = removeColor
	}

	baw := byteArrayWriter(*buf)
	(&protectedANSIWriter{
		w:    &baw,
		mode: pawMode,
	}).Write(msg)
	*buf = baw

	if event.stacktrace != "" && logger.StacktraceLevel <= event.level {
		lines := bytes.Split([]byte(event.stacktrace), []byte("\n"))
		if len(lines) > 1 {
			for _, line := range lines {
				*buf = append(*buf, "\n\t"...)
				*buf = append(*buf, line...)
			}
		}
		*buf = append(*buf, '\n')
	}
	*buf = append(*buf, '\n')
}

// LogEvent logs the event to the internal writer
func (logger *WriterLogger) LogEvent(event *Event) error {
	if logger.Level > event.level {
		return nil
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if !logger.Match(event) {
		return nil
	}
	var buf []byte
	logger.createMsg(&buf, event)
	_, err := logger.out.Write(buf)
	return err
}

// Match checks if the given event matches the logger's regexp expression
func (logger *WriterLogger) Match(event *Event) bool {
	if logger.regexp == nil {
		return true
	}
	if logger.regexp.Match([]byte(fmt.Sprintf("%s:%d:%s", event.filename, event.line, event.caller))) {
		return true
	}
	// Match on the non-colored msg - therefore strip out colors
	var msg []byte
	baw := byteArrayWriter(msg)
	(&protectedANSIWriter{
		w:    &baw,
		mode: removeColor,
	}).Write([]byte(event.msg))
	msg = baw
	return logger.regexp.Match(msg)
}

// Close the base logger
func (logger *WriterLogger) Close() {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.out != nil {
		logger.out.Close()
	}
}

// GetName returns empty for these provider loggers
func (logger *WriterLogger) GetName() string {
	return ""
}
