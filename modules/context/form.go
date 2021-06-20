// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// Forms a new enhancement of http.Request
type Forms http.Request

// Values returns http.Request values
func (f *Forms) Values() url.Values {
	return (*http.Request)(f).Form
}

// String returns request form as string
func (f *Forms) String(key string) (string, error) {
	return (*http.Request)(f).FormValue(key), nil
}

// Trimmed returns request form as string with trimed spaces left and right
func (f *Forms) Trimmed(key string) (string, error) {
	return strings.TrimSpace((*http.Request)(f).FormValue(key)), nil
}

// Strings returns request form as strings
func (f *Forms) Strings(key string) ([]string, error) {
	if (*http.Request)(f).Form == nil {
		if err := (*http.Request)(f).ParseMultipartForm(32 << 20); err != nil {
			return nil, err
		}
	}
	if v, ok := (*http.Request)(f).Form[key]; ok {
		return v, nil
	}
	return nil, errors.New("not exist")
}

// Escape returns request form as escaped string
func (f *Forms) Escape(key string) (string, error) {
	return template.HTMLEscapeString((*http.Request)(f).FormValue(key)), nil
}

// Int returns request form as int
func (f *Forms) Int(key string) (int, error) {
	return strconv.Atoi((*http.Request)(f).FormValue(key))
}

// Int32 returns request form as int32
func (f *Forms) Int32(key string) (int32, error) {
	v, err := strconv.ParseInt((*http.Request)(f).FormValue(key), 10, 32)
	return int32(v), err
}

// Int64 returns request form as int64
func (f *Forms) Int64(key string) (int64, error) {
	return strconv.ParseInt((*http.Request)(f).FormValue(key), 10, 64)
}

// Uint returns request form as uint
func (f *Forms) Uint(key string) (uint, error) {
	v, err := strconv.ParseUint((*http.Request)(f).FormValue(key), 10, 64)
	return uint(v), err
}

// Uint32 returns request form as uint32
func (f *Forms) Uint32(key string) (uint32, error) {
	v, err := strconv.ParseUint((*http.Request)(f).FormValue(key), 10, 32)
	return uint32(v), err
}

// Uint64 returns request form as uint64
func (f *Forms) Uint64(key string) (uint64, error) {
	return strconv.ParseUint((*http.Request)(f).FormValue(key), 10, 64)
}

// Bool returns request form as bool
func (f *Forms) Bool(key string) (bool, error) {
	return strconv.ParseBool((*http.Request)(f).FormValue(key))
}

// Float32 returns request form as float32
func (f *Forms) Float32(key string) (float32, error) {
	v, err := strconv.ParseFloat((*http.Request)(f).FormValue(key), 64)
	return float32(v), err
}

// Float64 returns request form as float64
func (f *Forms) Float64(key string) (float64, error) {
	return strconv.ParseFloat((*http.Request)(f).FormValue(key), 64)
}

// MustString returns request form as string with default
func (f *Forms) MustString(key string, defaults ...string) string {
	if v := (*http.Request)(f).FormValue(key); len(v) > 0 {
		return v
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

// MustTrimmed returns request form as string with default
func (f *Forms) MustTrimmed(key string, defaults ...string) string {
	return strings.TrimSpace(f.MustString(key, defaults...))
}

// MustStrings returns request form as strings with default
func (f *Forms) MustStrings(key string, defaults ...[]string) []string {
	if (*http.Request)(f).Form == nil {
		if err := (*http.Request)(f).ParseMultipartForm(32 << 20); err != nil {
			log.Error("ParseMultipartForm: %v", err)
			return []string{}
		}
	}

	if v, ok := (*http.Request)(f).Form[key]; ok {
		return v
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return []string{}
}

// MustEscape returns request form as escaped string with default
func (f *Forms) MustEscape(key string, defaults ...string) string {
	if v := (*http.Request)(f).FormValue(key); len(v) > 0 {
		return template.HTMLEscapeString(v)
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

// MustInt returns request form as int with default
func (f *Forms) MustInt(key string, defaults ...int) int {
	v, err := strconv.Atoi((*http.Request)(f).FormValue(key))
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return v
}

// MustInt32 returns request form as int32 with default
func (f *Forms) MustInt32(key string, defaults ...int32) int32 {
	v, err := strconv.ParseInt((*http.Request)(f).FormValue(key), 10, 32)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return int32(v)
}

// MustInt64 returns request form as int64 with default
func (f *Forms) MustInt64(key string, defaults ...int64) int64 {
	v, err := strconv.ParseInt((*http.Request)(f).FormValue(key), 10, 64)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return v
}

// MustUint returns request form as uint with default
func (f *Forms) MustUint(key string, defaults ...uint) uint {
	v, err := strconv.ParseUint((*http.Request)(f).FormValue(key), 10, 64)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return uint(v)
}

// MustUint32 returns request form as uint32 with default
func (f *Forms) MustUint32(key string, defaults ...uint32) uint32 {
	v, err := strconv.ParseUint((*http.Request)(f).FormValue(key), 10, 32)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return uint32(v)
}

// MustUint64 returns request form as uint64 with default
func (f *Forms) MustUint64(key string, defaults ...uint64) uint64 {
	v, err := strconv.ParseUint((*http.Request)(f).FormValue(key), 10, 64)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return v
}

// MustFloat32 returns request form as float32 with default
func (f *Forms) MustFloat32(key string, defaults ...float32) float32 {
	v, err := strconv.ParseFloat((*http.Request)(f).FormValue(key), 32)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return float32(v)
}

// MustFloat64 returns request form as float64 with default
func (f *Forms) MustFloat64(key string, defaults ...float64) float64 {
	v, err := strconv.ParseFloat((*http.Request)(f).FormValue(key), 64)
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return v
}

// MustBool returns request form as bool with default
func (f *Forms) MustBool(key string, defaults ...bool) bool {
	v, err := strconv.ParseBool((*http.Request)(f).FormValue(key))
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return v
}

// MustOptionalBool returns request form as OptionalBool with default
func (f *Forms) MustOptionalBool(key string, defaults ...util.OptionalBool) util.OptionalBool {
	value := (*http.Request)(f).FormValue(key)
	if len(value) == 0 {
		return util.OptionalBoolNone
	}
	v, err := strconv.ParseBool((*http.Request)(f).FormValue(key))
	if len(defaults) > 0 && err != nil {
		return defaults[0]
	}
	return util.OptionalBoolOf(v)
}
