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

func TestConsoleLoggerBadConfig(t *testing.T) {
	logger := NewConsoleLogger()

	err := logger.Init("{")
	assert.Contains(t, err.Error(), "JSON Unmarshal")
	logger.Close()
}

func TestConsoleLoggerMinimalConfig(t *testing.T) {
	for _, level := range Levels() {
		var written []byte
		var closed bool

		c := CallbackWriteCloser{
			callback: func(p []byte, close bool) {
				written = p
				closed = close
			},
		}
		prefix := ""
		flags := LstdFlags

		cw := NewConsoleLogger()
		realCW := cw.(*ConsoleLogger)
		cw.Init(fmt.Sprintf("{\"level\":\"%s\"}", level))
		nwc := realCW.out.(*nopWriteCloser)
		nwc.w = c

		assert.Equal(t, flags, realCW.Flags)
		assert.Equal(t, FromString(level), realCW.Level)
		assert.Equal(t, FromString(level), cw.GetLevel())
		assert.Equal(t, prefix, realCW.Prefix)
		assert.Equal(t, "", string(written))
		cw.Close()
		assert.False(t, closed)

	}
}

func TestConsoleLogger(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname

	cw := NewConsoleLogger()
	realCW := cw.(*ConsoleLogger)
	realCW.Colorize = false
	nwc := realCW.out.(*nopWriteCloser)
	nwc.w = c

	cw.Init(fmt.Sprintf("{\"expression\":\"FILENAME\",\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d}", prefix, level.String(), flags))

	assert.Equal(t, flags, realCW.Flags)
	assert.Equal(t, level, realCW.Level)
	assert.Equal(t, level, cw.GetLevel())

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

	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.False(t, closed)
	written = written[:0]

	event.level = DEBUG
	expected = ""
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.False(t, closed)

	event.level = TRACE
	expected = ""
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.False(t, closed)

	nonMatchEvent := Event{
		level:    INFO,
		msg:      "TEST MSG",
		caller:   "CALLER",
		filename: "FULL/FI_LENAME",
		line:     1,
		time:     date,
	}
	event.level = INFO
	expected = ""
	cw.LogEvent(&nonMatchEvent)
	assert.Equal(t, expected, string(written))
	assert.False(t, closed)

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.False(t, closed)
	written = written[:0]

	cw.Close()
	assert.False(t, closed)
}
