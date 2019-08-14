// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "time"

const (
	defaultMaxAge = time.Hour * 24 * 365
)

// HSTS is the configuration of HSTS
var HSTS = struct {
	Enabled              bool
	MaxAge               time.Duration
	IncludeSubDomains    bool
	SendPreloadDirective bool
}{
	Enabled:              false,
	MaxAge:               defaultMaxAge,
	IncludeSubDomains:    false,
	SendPreloadDirective: false,
}

func configHSTS() {
	sec := Cfg.Section("hsts")
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	HSTS.Enabled = true
	HSTS.MaxAge = sec.Key("MAX_AGE").MustDuration(defaultMaxAge)
	HSTS.IncludeSubDomains = sec.Key("INCLUDE_SUB_DOMAINS").MustBool()
	HSTS.SendPreloadDirective = sec.Key("SEND_PRELOAD_DIRECTIVE").MustBool()
}
