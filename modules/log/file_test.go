// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
)

func TestFileLoggerFails(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestFileLogger")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	// filename := filepath.Join(tmpDir, "test.log")

	fileLogger := NewFileLogger()
	// realFileLogger, ok := fileLogger.(*FileLogger)
	// assert.Equal(t, true, ok)

	// Fail if there is bad json
	err = fileLogger.Init("{")
	assert.Error(t, err)

	// Fail if there is no filename
	err = fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\"}", prefix, level.String(), flags, ""))
	assert.Error(t, err)

	// Fail if the file isn't a filename
	err = fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\"}", prefix, level.String(), flags, filepath.ToSlash(tmpDir)))
	assert.Error(t, err)
}

func TestFileLogger(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestFileLogger")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	filename := filepath.Join(tmpDir, "test.log")

	fileLogger := NewFileLogger()
	realFileLogger, ok := fileLogger.(*FileLogger)
	assert.Equal(t, true, ok)

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

	fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\",\"maxsize\":%d,\"compress\":false}", prefix, level.String(), flags, filepath.ToSlash(filename), len(expected)*2))

	assert.Equal(t, flags, realFileLogger.Flags)
	assert.Equal(t, level, realFileLogger.Level)
	assert.Equal(t, level, fileLogger.GetLevel())

	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = DEBUG
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = TRACE
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = WARN
	expected += fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	// Should rotate
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename + fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), 1))
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	assert.Equal(t, expected, string(logData))

	for num := 2; num <= 999; num++ {
		file, err := os.OpenFile(filename+fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), num), os.O_RDONLY|os.O_CREATE, 0o666)
		assert.NoError(t, err)
		file.Close()
	}
	err = realFileLogger.DoRotate()
	assert.Error(t, err)

	expected += fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	// Should fail to rotate
	expected += fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	fileLogger.Close()
}

func TestCompressFileLogger(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestFileLogger")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	filename := filepath.Join(tmpDir, "test.log")

	fileLogger := NewFileLogger()
	realFileLogger, ok := fileLogger.(*FileLogger)
	assert.Equal(t, true, ok)

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

	fileLogger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"filename\":\"%s\",\"maxsize\":%d,\"compress\":true}", prefix, level.String(), flags, filepath.ToSlash(filename), len(expected)*2))

	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	event.level = WARN
	expected += fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	fileLogger.LogEvent(&event)
	fileLogger.Flush()
	logData, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(logData))

	// Should rotate
	fileLogger.LogEvent(&event)
	fileLogger.Flush()

	for num := 2; num <= 999; num++ {
		file, err := os.OpenFile(filename+fmt.Sprintf(".%s.%03d.gz", time.Now().Format("2006-01-02"), num), os.O_RDONLY|os.O_CREATE, 0o666)
		assert.NoError(t, err)
		file.Close()
	}
	err = realFileLogger.DoRotate()
	assert.Error(t, err)
}

func TestCompressOldFile(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestFileLogger")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)
	fname := filepath.Join(tmpDir, "test")
	nonGzip := filepath.Join(tmpDir, "test-nonGzip")

	f, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0o660)
	assert.NoError(t, err)
	ng, err := os.OpenFile(nonGzip, os.O_CREATE|os.O_WRONLY, 0o660)
	assert.NoError(t, err)

	for i := 0; i < 999; i++ {
		f.WriteString("This is a test file\n")
		ng.WriteString("This is a test file\n")
	}
	f.Close()
	ng.Close()

	err = compressOldLogFile(fname, -1)
	assert.NoError(t, err)

	_, err = os.Lstat(fname + ".gz")
	assert.NoError(t, err)

	f, err = os.Open(fname + ".gz")
	assert.NoError(t, err)
	zr, err := gzip.NewReader(f)
	assert.NoError(t, err)
	data, err := ioutil.ReadAll(zr)
	assert.NoError(t, err)
	original, err := ioutil.ReadFile(nonGzip)
	assert.NoError(t, err)
	assert.Equal(t, original, data)
}
