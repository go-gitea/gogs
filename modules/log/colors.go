// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

const escape = "\033"

// ColorAttribute defines a single SGR Code
type ColorAttribute int

// Base ColorAttributes
const (
	Reset ColorAttribute = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground text colors
const (
	FgBlack ColorAttribute = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Foreground Hi-Intensity text colors
const (
	FgHiBlack ColorAttribute = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background text colors
const (
	BgBlack ColorAttribute = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
)

// Background Hi-Intensity text colors
const (
	BgHiBlack ColorAttribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

var colorAttributeToString = map[ColorAttribute]string{
	Reset:        "Reset",
	Bold:         "Bold",
	Faint:        "Faint",
	Italic:       "Italic",
	Underline:    "Underline",
	BlinkSlow:    "BlinkSlow",
	BlinkRapid:   "BlinkRapid",
	ReverseVideo: "ReverseVideo",
	Concealed:    "Concealed",
	CrossedOut:   "CrossedOut",
	FgBlack:      "FgBlack",
	FgRed:        "FgRed",
	FgGreen:      "FgGreen",
	FgYellow:     "FgYellow",
	FgBlue:       "FgBlue",
	FgMagenta:    "FgMagenta",
	FgCyan:       "FgCyan",
	FgWhite:      "FgWhite",
	FgHiBlack:    "FgHiBlack",
	FgHiRed:      "FgHiRed",
	FgHiGreen:    "FgHiGreen",
	FgHiYellow:   "FgHiYellow",
	FgHiBlue:     "FgHiBlue",
	FgHiMagenta:  "FgHiMagenta",
	FgHiCyan:     "FgHiCyan",
	FgHiWhite:    "FgHiWhite",
	BgBlack:      "BgBlack",
	BgRed:        "BgRed",
	BgGreen:      "BgGreen",
	BgYellow:     "BgYellow",
	BgBlue:       "BgBlue",
	BgMagenta:    "BgMagenta",
	BgCyan:       "BgCyan",
	BgWhite:      "BgWhite",
	BgHiBlack:    "BgHiBlack",
	BgHiRed:      "BgHiRed",
	BgHiGreen:    "BgHiGreen",
	BgHiYellow:   "BgHiYellow",
	BgHiBlue:     "BgHiBlue",
	BgHiMagenta:  "BgHiMagenta",
	BgHiCyan:     "BgHiCyan",
	BgHiWhite:    "BgHiWhite",
}

func (c *ColorAttribute) String() string {
	return colorAttributeToString[*c]
}

var colorAttributeFromString = map[string]ColorAttribute{}

// ColorAttributeFromString will return a ColorAttribute given a string
func ColorAttributeFromString(from string) ColorAttribute {
	lowerFrom := strings.TrimSpace(strings.ToLower(from))
	return colorAttributeFromString[lowerFrom]
}

// ColorString converts a list of ColorAttributes to a color string
func ColorString(attrs ...ColorAttribute) string {
	return string(ColorBytes(attrs...))
}

// ColorBytes converts a list of ColorAttributes to a byte array
func ColorBytes(attrs ...ColorAttribute) []byte {
	bytes := make([]byte, 0, 20)
	bytes = append(bytes, escape[0], '[')
	if len(attrs) > 0 {
		bytes = append(bytes, strconv.Itoa(int(attrs[0]))...)
		for _, a := range attrs[1:] {
			bytes = append(bytes, ';')
			bytes = append(bytes, strconv.Itoa(int(a))...)
		}
	} else {
		bytes = append(bytes, strconv.Itoa(int(Bold))...)
	}
	bytes = append(bytes, 'm')
	return bytes
}

var levelToColor = map[Level][]byte{
	TRACE:    ColorBytes(Bold, FgCyan),
	DEBUG:    ColorBytes(Bold, FgBlue),
	INFO:     ColorBytes(Bold, FgGreen),
	WARN:     ColorBytes(Bold, FgYellow),
	ERROR:    ColorBytes(Bold, FgRed),
	CRITICAL: ColorBytes(Bold, BgMagenta),
	FATAL:    ColorBytes(Bold, BgRed),
	NONE:     ColorBytes(Reset),
}

var (
	resetBytes   = ColorBytes(Reset)
	fgCyanBytes  = ColorBytes(FgCyan)
	fgGreenBytes = ColorBytes(FgGreen)
	fgBoldBytes  = ColorBytes(Bold)
)

type protectedANSIWriterMode int

const (
	escapeAll protectedANSIWriterMode = iota
	allowColor
	removeColor
)

type protectedANSIWriter struct {
	w    io.Writer
	mode protectedANSIWriterMode
}

// Write will protect against unusual characters
func (c *protectedANSIWriter) Write(bytes []byte) (int, error) {
	end := len(bytes)
	totalWritten := 0
normalLoop:
	for i := 0; i < end; {
		lasti := i

		if c.mode == escapeAll {
			for i < end && (bytes[i] >= ' ' || bytes[i] == '\n' || bytes[i] == '\t') {
				i++
			}
		} else {
			// Allow tabs if we're not escaping everything
			for i < end && (bytes[i] >= ' ' || bytes[i] == '\t') {
				i++
			}
		}

		if i > lasti {
			written, err := c.w.Write(bytes[lasti:i])
			totalWritten += written
			if err != nil {
				return totalWritten, err
			}

		}
		if i >= end {
			break
		}

		// If we're not just escaping all we should prefix all newlines with a \t
		if c.mode != escapeAll {
			if bytes[i] == '\n' {
				written, err := c.w.Write([]byte{'\n', '\t'})
				if written > 0 {
					totalWritten++
				}
				if err != nil {
					return totalWritten, err
				}
				i++
				continue normalLoop
			}

			if bytes[i] == escape[0] && i+1 < end && bytes[i+1] == '[' {
				for j := i + 2; j < end; j++ {
					if bytes[j] >= '0' && bytes[j] <= '9' {
						continue
					}
					if bytes[j] == ';' {
						continue
					}
					if bytes[j] == 'm' {
						if c.mode == allowColor {
							written, err := c.w.Write(bytes[i : j+1])
							totalWritten += written
							if err != nil {
								return totalWritten, err
							}
						} else {
							totalWritten = j
						}
						i = j + 1
						continue normalLoop
					}
					break
				}
			}
		}

		// Process naughty character
		if _, err := fmt.Fprintf(c.w, `\%#o03d`, bytes[i]); err != nil {
			return totalWritten, err
		}
		i++
		totalWritten++
	}
	return totalWritten, nil
}

// ColorSprintf returns a colored string from a format and arguments
// arguments will be wrapped in ColoredValues to protect against color spoofing
func ColorSprintf(format string, args ...interface{}) string {
	if len(args) > 0 {
		v := make([]interface{}, len(args))
		for i := 0; i < len(v); i++ {
			v[i] = NewColoredValuePointer(&args[i])
		}
		return fmt.Sprintf(format, v...)
	}
	return format
}

// ColorFprintf will write to the provided writer similar to ColorSprintf
func ColorFprintf(w io.Writer, format string, args ...interface{}) (int, error) {
	if len(args) > 0 {
		v := make([]interface{}, len(args))
		for i := 0; i < len(v); i++ {
			v[i] = NewColoredValuePointer(&args[i])
		}
		return fmt.Fprintf(w, format, v...)
	}
	return fmt.Fprint(w, format)
}

// ColorFormatted structs provide their own colored string when formatted with ColorSprintf
type ColorFormatted interface {
	// ColorFormat provides the colored representation of the value
	ColorFormat(s fmt.State)
}

var colorFormattedType = reflect.TypeOf((*ColorFormatted)(nil)).Elem()

// ColoredValue will Color the provided value
type ColoredValue struct {
	colorBytes *[]byte
	resetBytes *[]byte
	Value      *interface{}
}

// NewColoredValue is a helper function to create a ColoredValue from a Value
// If no color is provided it defaults to Bold with standard Reset
// If a ColoredValue is provided it is not changed
func NewColoredValue(value interface{}, color ...ColorAttribute) *ColoredValue {
	return NewColoredValuePointer(&value, color...)
}

// NewColoredValuePointer is a helper function to create a ColoredValue from a Value Pointer
// If no color is provided it defaults to Bold with standard Reset
// If a ColoredValue is provided it is not changed
func NewColoredValuePointer(value *interface{}, color ...ColorAttribute) *ColoredValue {
	if val, ok := (*value).(*ColoredValue); ok {
		return val
	}
	if len(color) > 0 {
		bytes := ColorBytes(color...)
		return &ColoredValue{
			colorBytes: &bytes,
			resetBytes: &resetBytes,
			Value:      value,
		}
	}
	return &ColoredValue{
		colorBytes: &fgBoldBytes,
		resetBytes: &resetBytes,
		Value:      value,
	}
}

// NewColoredValueBytes creates a value from the provided value with color bytes
// If a ColoredValue is provided it is not changed
func NewColoredValueBytes(value interface{}, colorBytes *[]byte) *ColoredValue {
	if val, ok := value.(*ColoredValue); ok {
		return val
	}
	return &ColoredValue{
		colorBytes: colorBytes,
		resetBytes: &resetBytes,
		Value:      &value,
	}
}

// NewColoredIDValue is a helper function to create a ColoredValue from a Value
// The Value will be colored with FgCyan
// If a ColoredValue is provided it is not changed
func NewColoredIDValue(value interface{}) *ColoredValue {
	return NewColoredValueBytes(value, &fgCyanBytes)
}

// Format will format the provided value and protect against ANSI color spoofing within the value
// If the wrapped value is ColorFormatted and the format is "%-v" then its ColorString will
// be used. It is presumed that this ColorString is safe.
func (cv *ColoredValue) Format(s fmt.State, c rune) {
	if c == 'v' && s.Flag('-') {
		if val, ok := (*cv.Value).(ColorFormatted); ok {
			val.ColorFormat(s)
			return
		}
		v := reflect.ValueOf(*cv.Value)
		t := v.Type()

		if reflect.PtrTo(t).Implements(colorFormattedType) {
			vp := reflect.New(t)
			vp.Elem().Set(v)
			val := vp.Interface().(ColorFormatted)
			val.ColorFormat(s)
			return
		}
	}
	s.Write(*cv.colorBytes)
	fmt.Fprintf(&protectedANSIWriter{w: s}, fmtString(s, c), *(cv.Value))
	s.Write(*cv.resetBytes)
}

// SetColorBytes will allow a user to set the colorBytes of a colored value
func (cv *ColoredValue) SetColorBytes(colorBytes []byte) {
	cv.colorBytes = &colorBytes
}

// SetColorBytesPointer will allow a user to set the colorBytes pointer of a colored value
func (cv *ColoredValue) SetColorBytesPointer(colorBytes *[]byte) {
	cv.colorBytes = colorBytes
}

// SetResetBytes will allow a user to set the resetBytes pointer of a colored value
func (cv *ColoredValue) SetResetBytes(resetBytes []byte) {
	cv.resetBytes = &resetBytes
}

// SetResetBytesPointer will allow a user to set the resetBytes pointer of a colored value
func (cv *ColoredValue) SetResetBytesPointer(resetBytes *[]byte) {
	cv.resetBytes = resetBytes
}

func fmtString(s fmt.State, c rune) string {
	var width, precision string
	base := make([]byte, 0, 8)
	base = append(base, '%')
	for _, c := range []byte(" +-#0") {
		if s.Flag(int(c)) {
			base = append(base, c)
		}
	}
	if w, ok := s.Width(); ok {
		width = strconv.Itoa(w)
	}
	if p, ok := s.Precision(); ok {
		precision = "." + strconv.Itoa(p)
	}
	return fmt.Sprintf("%s%s%s%c", base, width, precision, c)
}

func init() {
	for attr, from := range colorAttributeToString {
		colorAttributeFromString[strings.ToLower(from)] = attr
	}
}
