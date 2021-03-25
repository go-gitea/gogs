// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type CallbackWriteCloser struct {
	callback func([]byte, bool)
}

func (c CallbackWriteCloser) Write(p []byte) (int, error) {
	c.callback(p, false)
	return len(p), nil
}

func (c CallbackWriteCloser) Close() error {
	c.callback(nil, true)
	return nil
}

func TestBaseLogger(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := "TestPrefix "
	b := WriterLogger{
		out:    c,
		Level:  INFO,
		Flags:  LstdFlags | LUTC,
		Prefix: prefix,
	}
	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	dateString := date.UTC().Format("2006/01/02 15:04:05")

	event := Event{
		level:    INFO,
		msg:      "TEST MSG",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	assert.Equal(t, INFO, b.GetLevel())

	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = DEBUG
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)

	event.level = TRACE
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = ERROR
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = CRITICAL
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	b.Close()
	assert.Equal(t, true, closed)
}

func TestBaseLoggerDated(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := ""
	b := WriterLogger{
		out:    c,
		Level:  WARN,
		Flags:  Ldate | Ltime | Lmicroseconds | Lshortfile | Llevel,
		Prefix: prefix,
	}

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 115, location)

	dateString := date.Format("2006/01/02 15:04:05.000000")

	event := Event{
		level:    WARN,
		msg:      "TEST MESSAGE TEST\n",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	assert.Equal(t, WARN, b.GetLevel())

	expected := fmt.Sprintf("%s%s %s:%d [%s] %s", prefix, dateString, "FILENAME", event.line, strings.ToUpper(event.level.String()), event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = INFO
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = ERROR
	expected = fmt.Sprintf("%s%s %s:%d [%s] %s", prefix, dateString, "FILENAME", event.line, strings.ToUpper(event.level.String()), event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = DEBUG
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = CRITICAL
	expected = fmt.Sprintf("%s%s %s:%d [%s] %s", prefix, dateString, "FILENAME", event.line, strings.ToUpper(event.level.String()), event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = TRACE
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	b.Close()
	assert.Equal(t, true, closed)
}

func TestBaseLoggerMultiLineNoFlagsRegexp(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := ""
	b := WriterLogger{
		Level:           DEBUG,
		StacktraceLevel: ERROR,
		Flags:           -1,
		Prefix:          prefix,
		Expression:      "FILENAME",
	}
	b.NewWriterLogger(c)

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 115, location)

	event := Event{
		level:    DEBUG,
		msg:      "TEST\nMESSAGE\nTEST",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	assert.Equal(t, DEBUG, b.GetLevel())

	expected := "TEST\n\tMESSAGE\n\tTEST\n"
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.filename = "ELSEWHERE"

	b.LogEvent(&event)
	assert.Equal(t, "", string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.caller = "FILENAME"
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event = Event{
		level:    DEBUG,
		msg:      "TEST\nFILENAME\nTEST",
		caller:   "CALLER",
		filename: "FULL/ELSEWHERE",
		line:     1,
		time:     date,
	}
	expected = "TEST\n\tFILENAME\n\tTEST\n"
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]
}

func TestBrokenRegexp(t *testing.T) {
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			closed = close
		},
	}

	b := WriterLogger{
		Level:           DEBUG,
		StacktraceLevel: ERROR,
		Flags:           -1,
		Prefix:          prefix,
		Expression:      "\\",
	}
	b.NewWriterLogger(c)
	assert.Empty(t, b.regexp)
	b.Close()
	assert.Equal(t, true, closed)
}
