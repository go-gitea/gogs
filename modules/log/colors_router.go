// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"time"
)

var statusToColor = map[int][]byte{
	100: ColorBytes(Bold),
	200: ColorBytes(FgGreen),
	300: ColorBytes(FgYellow),
	304: ColorBytes(FgCyan),
	400: ColorBytes(Bold, FgRed),
	401: ColorBytes(Bold, FgMagenta),
	403: ColorBytes(Bold, FgMagenta),
	500: ColorBytes(Bold, BgRed),
}

// ColoredStatus adds colors for HTTP status
func ColoredStatus(status int, s ...string) *ColoredValue {
	color, ok := statusToColor[status]
	if !ok {
		color, ok = statusToColor[(status/100)*100]
	}
	if !ok {
		color = fgBoldBytes
	}
	if len(s) > 0 {
		return NewColoredValueBytes(s[0], &color)
	}
	return NewColoredValueBytes(status, &color)
}

var methodToColor = map[string][]byte{
	"GET":    ColorBytes(FgBlue),
	"POST":   ColorBytes(FgGreen),
	"DELETE": ColorBytes(FgRed),
	"PATCH":  ColorBytes(FgCyan),
	"PUT":    ColorBytes(FgYellow, Faint),
	"HEAD":   ColorBytes(FgBlue, Faint),
}

// ColoredMethod adds colors for HTTP methods on log
func ColoredMethod(method string) *ColoredValue {
	color, ok := methodToColor[method]
	if !ok {
		return NewColoredValueBytes(method, &fgBoldBytes)
	}
	return NewColoredValueBytes(method, &color)
}

var (
	durations = []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
		5 * time.Second,
		10 * time.Second,
	}

	durationColors = [][]byte{
		ColorBytes(FgGreen),
		ColorBytes(Bold),
		ColorBytes(FgYellow),
		ColorBytes(FgRed, Bold),
		ColorBytes(BgRed),
	}

	wayTooLong = ColorBytes(BgMagenta)
)

// ColoredTime adds colors for time on log
func ColoredTime(duration time.Duration) *ColoredValue {
	for i, k := range durations {
		if duration < k {
			return NewColoredValueBytes(duration, &durationColors[i])
		}
	}
	return NewColoredValueBytes(duration, &wayTooLong)
}
