// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"github.com/goccy/go-json"
)

var (
	prefix    string
	slowTest  = 10 * time.Second
	slowFlush = 5 * time.Second
)

// TestLogger is a logger which will write to the testing log
type TestLogger struct {
	log.WriterLogger
}

var writerCloser = &testLoggerWriterCloser{}

type testLoggerWriterCloser struct {
	sync.RWMutex
	t []*testing.TB
}

func (w *testLoggerWriterCloser) setT(t *testing.TB) {
	w.Lock()
	w.t = append(w.t, t)
	w.Unlock()
}

func (w *testLoggerWriterCloser) Write(p []byte) (int, error) {
	w.RLock()
	var t *testing.TB
	if len(w.t) > 0 {
		t = w.t[len(w.t)-1]
	}
	w.RUnlock()
	if t != nil && *t != nil {
		if len(p) > 0 && p[len(p)-1] == '\n' {
			p = p[:len(p)-1]
		}

		defer func() {
			err := recover()
			if err == nil {
				return
			}
			var errString string
			errErr, ok := err.(error)
			if ok {
				errString = errErr.Error()
			} else {
				errString, ok = err.(string)
			}
			if !ok {
				panic(err)
			}
			if !strings.HasPrefix(errString, "Log in goroutine after ") {
				panic(err)
			}
		}()

		(*t).Log(string(p))
		return len(p), nil
	}
	return len(p), nil
}

func (w *testLoggerWriterCloser) Close() error {
	w.Lock()
	if len(w.t) > 0 {
		w.t = w.t[:len(w.t)-1]
	}
	w.Unlock()
	return nil
}

// PrintCurrentTest prints the current test to os.Stdout
func PrintCurrentTest(t testing.TB, skip ...int) func() {
	start := time.Now()
	actualSkip := 1
	if len(skip) > 0 {
		actualSkip = skip[0]
	}
	_, filename, line, _ := runtime.Caller(actualSkip)

	if log.CanColorStdout {
		fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", fmt.Formatter(log.NewColoredValue(t.Name())), strings.TrimPrefix(filename, prefix), line)
	} else {
		fmt.Fprintf(os.Stdout, "=== %s (%s:%d)\n", t.Name(), strings.TrimPrefix(filename, prefix), line)
	}
	writerCloser.setT(&t)
	return func() {
		took := time.Since(start)
		if took > slowTest {
			if log.CanColorStdout {
				fmt.Fprintf(os.Stdout, "+++ %s is a slow test (took %v)\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgYellow)), fmt.Formatter(log.NewColoredValue(took, log.Bold, log.FgYellow)))
			} else {
				fmt.Fprintf(os.Stdout, "+++ %s is a slow tets (took %v)\n", t.Name(), took)
			}
		}
		timer := time.AfterFunc(slowFlush, func() {
			if log.CanColorStdout {
				fmt.Fprintf(os.Stdout, "+++ %s ... still flushing after %v ...\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgRed)), slowFlush)
			} else {
				fmt.Fprintf(os.Stdout, "+++ %s ... still flushing after %v ...\n", t.Name(), slowFlush)
			}
		})
		if err := queue.GetManager().FlushAll(context.Background(), -1); err != nil {
			t.Errorf("Flushing queues failed with error %v", err)
		}
		timer.Stop()
		flushTook := time.Since(start) - took
		if flushTook > slowFlush {
			if log.CanColorStdout {
				fmt.Fprintf(os.Stdout, "+++ %s had a slow clean-up flush (took %v)\n", fmt.Formatter(log.NewColoredValue(t.Name(), log.Bold, log.FgRed)), fmt.Formatter(log.NewColoredValue(flushTook, log.Bold, log.FgRed)))
			} else {
				fmt.Fprintf(os.Stdout, "+++ %s had a slow clean-up flush (took %v)\n", t.Name(), flushTook)
			}
		}
		_ = writerCloser.Close()
	}
}

// Printf takes a format and args and prints the string to os.Stdout
func Printf(format string, args ...interface{}) {
	if log.CanColorStdout {
		for i := 0; i < len(args); i++ {
			args[i] = log.NewColoredValue(args[i])
		}
	}
	fmt.Fprintf(os.Stdout, "\t"+format, args...)
}

// NewTestLogger creates a TestLogger as a log.LoggerProvider
func NewTestLogger() log.LoggerProvider {
	logger := &TestLogger{}
	logger.Colorize = log.CanColorStdout
	logger.Level = log.TRACE
	return logger
}

// Init inits connection writer with json config.
// json config only need key "level".
func (log *TestLogger) Init(config string) error {
	err := json.Unmarshal([]byte(config), log)
	if err != nil {
		return err
	}
	log.NewWriterLogger(writerCloser)
	return nil
}

// Flush when log should be flushed
func (log *TestLogger) Flush() {
}

//ReleaseReopen does nothing
func (log *TestLogger) ReleaseReopen() error {
	return nil
}

// GetName returns the default name for this implementation
func (log *TestLogger) GetName() string {
	return "test"
}

func init() {
	log.Register("test", NewTestLogger)
	_, filename, _, _ := runtime.Caller(0)
	prefix = strings.TrimSuffix(filename, "integrations/testlogger.go")
}
