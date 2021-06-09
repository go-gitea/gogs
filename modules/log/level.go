// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-json"
)

// Level is the level of the logger
type Level int

const (
	// TRACE represents the lowest log level
	TRACE Level = iota
	// DEBUG is for debug logging
	DEBUG
	// INFO is for information
	INFO
	// WARN is for warning information
	WARN
	// ERROR is for error reporting
	ERROR
	// CRITICAL is for critical errors
	CRITICAL
	// FATAL is for fatal errors
	FATAL
	// NONE is for no logging
	NONE
)

var toString = map[Level]string{
	TRACE:    "trace",
	DEBUG:    "debug",
	INFO:     "info",
	WARN:     "warn",
	ERROR:    "error",
	CRITICAL: "critical",
	FATAL:    "fatal",
	NONE:     "none",
}

var toLevel = map[string]Level{
	"trace":    TRACE,
	"debug":    DEBUG,
	"info":     INFO,
	"warn":     WARN,
	"error":    ERROR,
	"critical": CRITICAL,
	"fatal":    FATAL,
	"none":     NONE,
}

// Levels returns all the possible logging levels
func Levels() []string {
	keys := make([]string, 0)
	for key := range toLevel {
		keys = append(keys, key)
	}
	return keys
}

func (l Level) String() string {
	s, ok := toString[l]
	if ok {
		return s
	}
	return "info"
}

// Color returns the color string for this Level
func (l Level) Color() *[]byte {
	color, ok := levelToColor[l]
	if ok {
		return &(color)
	}
	none := levelToColor[NONE]
	return &none
}

// MarshalJSON takes a Level and turns it into text
func (l Level) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[l])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// FromString takes a level string and returns a Level
func FromString(level string) Level {
	temp, ok := toLevel[strings.ToLower(level)]
	if !ok {
		return INFO
	}
	return temp
}

// UnmarshalJSON takes text and turns it into a Level
func (l *Level) UnmarshalJSON(b []byte) error {
	var tmp interface{}
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Err: %v", err)
		return err
	}

	switch v := tmp.(type) {
	case string:
		*l = FromString(v)
	case int:
		*l = FromString(Level(v).String())
	default:
		*l = INFO
	}
	return nil
}
