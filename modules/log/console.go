// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-json"
)

// CanColorStdout reports if we can color the Stdout
// Although we could do terminal sniffing and the like - in reality
// most tools on *nix are happy to display ansi colors.
// We will terminal sniff on Windows in console_windows.go
var CanColorStdout = true

// CanColorStderr reports if we can color the Stderr
var CanColorStderr = true

type nopWriteCloser struct {
	w io.WriteCloser
}

func (n *nopWriteCloser) Write(p []byte) (int, error) {
	return n.w.Write(p)
}

func (n *nopWriteCloser) Close() error {
	return nil
}

// ConsoleLogger implements LoggerProvider and writes messages to terminal.
type ConsoleLogger struct {
	WriterLogger
	Stderr bool `json:"stderr"`
}

// NewConsoleLogger create ConsoleLogger returning as LoggerProvider.
func NewConsoleLogger() LoggerProvider {
	log := &ConsoleLogger{}
	log.NewWriterLogger(&nopWriteCloser{
		w: os.Stdout,
	})
	return log
}

// Init inits connection writer with json config.
// json config only need key "level".
func (log *ConsoleLogger) Init(config string) error {
	err := json.Unmarshal([]byte(config), log)
	if err != nil {
		return fmt.Errorf("JSON Unmarshal: %v", err)
	}
	if log.Stderr {
		log.NewWriterLogger(&nopWriteCloser{
			w: os.Stderr,
		})
	} else {
		log.NewWriterLogger(log.out)
	}
	return nil
}

// Flush when log should be flushed
func (log *ConsoleLogger) Flush() {
}

// ReleaseReopen causes the console logger to reconnect to os.Stdout
func (log *ConsoleLogger) ReleaseReopen() error {
	if log.Stderr {
		log.NewWriterLogger(&nopWriteCloser{
			w: os.Stderr,
		})
	} else {
		log.NewWriterLogger(&nopWriteCloser{
			w: os.Stdout,
		})
	}
	return nil
}

// GetName returns the default name for this implementation
func (log *ConsoleLogger) GetName() string {
	return "console"
}

func init() {
	Register("console", NewConsoleLogger)
}
