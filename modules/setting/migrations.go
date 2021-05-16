// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
)

var (
	// Migrations settings
	Migrations = struct {
		MaxAttempts        int
		RetryBackoff       int
		AllowedDomains     []string
		BlockedDomains     []string
		AllowLocalNetworks bool
		SkipTLSVerify      bool
	}{
		MaxAttempts:  3,
		RetryBackoff: 3,
	}
)

func newMigrationsService() {
	sec := Cfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowedDomains = sec.Key("ALLOWED_DOMAINS").Strings(",")
	for i := range Migrations.AllowedDomains {
		Migrations.AllowedDomains[i] = strings.ToLower(Migrations.AllowedDomains[i])
	}
	Migrations.BlockedDomains = sec.Key("BLOCKED_DOMAINS").Strings(",")
	for i := range Migrations.BlockedDomains {
		Migrations.BlockedDomains[i] = strings.ToLower(Migrations.BlockedDomains[i])
	}

	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
}
