// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func listenReadAndClose(t *testing.T, l net.Listener, expected string) {
	conn, err := l.Accept()
	assert.NoError(t, err)
	defer conn.Close()
	written, err := ioutil.ReadAll(conn)

	assert.NoError(t, err)
	assert.Equal(t, expected, string(written))
}

func TestConnLogger(t *testing.T) {
	protocol := "tcp"
	address := ":3099"

	l, err := net.Listen(protocol, address)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname

	logger := NewConn()
	connLogger := logger.(*ConnLogger)

	logger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"reconnectOnMsg\":%t,\"reconnect\":%t,\"net\":\"%s\",\"addr\":\"%s\"}", prefix, level.String(), flags, true, true, protocol, address))

	assert.Equal(t, flags, connLogger.Flags)
	assert.Equal(t, level, connLogger.Level)
	assert.Equal(t, level, logger.GetLevel())

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
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		listenReadAndClose(t, l, expected)
	}()
	go func() {
		defer wg.Done()
		err := logger.LogEvent(&event)
		assert.NoError(t, err)
	}()
	wg.Wait()

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	wg.Add(2)
	go func() {
		defer wg.Done()
		listenReadAndClose(t, l, expected)
	}()
	go func() {
		defer wg.Done()
		err := logger.LogEvent(&event)
		assert.NoError(t, err)
	}()
	wg.Wait()

	logger.Close()
}

func TestConnLoggerBadConfig(t *testing.T) {
	logger := NewConn()

	err := logger.Init("{")
	assert.Contains(t, err.Error(), "JSON Unmarshal")
	logger.Close()
}

func TestConnLoggerCloseBeforeSend(t *testing.T) {
	protocol := "tcp"
	address := ":3099"

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname

	logger := NewConn()

	logger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"reconnectOnMsg\":%t,\"reconnect\":%t,\"net\":\"%s\",\"addr\":\"%s\"}", prefix, level.String(), flags, false, false, protocol, address))
	logger.Close()
}

func TestConnLoggerFailConnect(t *testing.T) {
	protocol := "tcp"
	address := ":3099"

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname

	logger := NewConn()

	logger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"reconnectOnMsg\":%t,\"reconnect\":%t,\"net\":\"%s\",\"addr\":\"%s\"}", prefix, level.String(), flags, false, false, protocol, address))

	assert.Equal(t, level, logger.GetLevel())

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	//dateString := date.UTC().Format("2006/01/02 15:04:05")

	event := Event{
		level:    INFO,
		msg:      "TEST MSG",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	err := logger.LogEvent(&event)
	assert.Error(t, err)

	logger.Close()
}

func TestConnLoggerClose(t *testing.T) {
	protocol := "tcp"
	address := ":3099"

	l, err := net.Listen(protocol, address)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname

	logger := NewConn()
	connLogger := logger.(*ConnLogger)

	logger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"reconnectOnMsg\":%t,\"reconnect\":%t,\"net\":\"%s\",\"addr\":\"%s\"}", prefix, level.String(), flags, false, false, protocol, address))

	assert.Equal(t, flags, connLogger.Flags)
	assert.Equal(t, level, connLogger.Level)
	assert.Equal(t, level, logger.GetLevel())
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
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := logger.LogEvent(&event)
		assert.NoError(t, err)
		logger.Close()
	}()
	go func() {
		defer wg.Done()
		listenReadAndClose(t, l, expected)
	}()
	wg.Wait()

	logger = NewConn()
	connLogger = logger.(*ConnLogger)

	logger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"reconnectOnMsg\":%t,\"reconnect\":%t,\"net\":\"%s\",\"addr\":\"%s\"}", prefix, level.String(), flags, false, true, protocol, address))

	assert.Equal(t, flags, connLogger.Flags)
	assert.Equal(t, level, connLogger.Level)
	assert.Equal(t, level, logger.GetLevel())

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	wg.Add(2)
	go func() {
		defer wg.Done()
		listenReadAndClose(t, l, expected)
	}()
	go func() {
		defer wg.Done()
		err := logger.LogEvent(&event)
		assert.NoError(t, err)
		logger.Close()

	}()
	wg.Wait()
	logger.Flush()
	logger.Close()
}
