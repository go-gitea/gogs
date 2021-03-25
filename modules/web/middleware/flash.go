// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import "net/url"

// flashes enumerates all the flash types
const (
	SuccessFlash = "SuccessMsg"
	ErrorFlash   = "ErrorMsg"
	WarnFlash    = "WarningMsg"
	InfoFlash    = "InfoMsg"
)

// FlashNow FIXME:
var FlashNow bool

// Flash represents a one time data transfer between two requests.
type Flash struct {
	DataStore
	url.Values
	ErrorMsg, WarningMsg, InfoMsg, SuccessMsg string
}

func (f *Flash) set(name, msg string, current ...bool) {
	if f.Values == nil {
		f.Values = make(map[string][]string)
	}
	isShow := false
	if (len(current) == 0 && FlashNow) ||
		(len(current) > 0 && current[0]) {
		isShow = true
	}

	if isShow {
		f.GetData()["Flash"] = f
	} else {
		f.Set(name, msg)
	}
}

// Error sets error message
func (f *Flash) Error(msg string, current ...bool) {
	f.ErrorMsg = msg
	f.set("error", msg, current...)
}

// Warning sets warning message
func (f *Flash) Warning(msg string, current ...bool) {
	f.WarningMsg = msg
	f.set("warning", msg, current...)
}

// Info sets info message
func (f *Flash) Info(msg string, current ...bool) {
	f.InfoMsg = msg
	f.set("info", msg, current...)
}

// Success sets success message
func (f *Flash) Success(msg string, current ...bool) {
	f.SuccessMsg = msg
	f.set("success", msg, current...)
}
