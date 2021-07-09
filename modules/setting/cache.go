// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Cache represents cache settings
type Cache struct {
	Enabled       bool
	Adapter       string
	AdapterConfig string
	Interval      int
	Conn          string
	TTL           time.Duration `ini:"ITEM_TTL"`
}

var (
	// CacheService the global cache
	CacheService = struct {
		Cache `ini:"cache"`

		LastCommit struct {
			Enabled      bool
			TTL          time.Duration `ini:"ITEM_TTL"`
			CommitsCount int64
		} `ini:"cache.last_commit"`
	}{
		Cache: Cache{
			Enabled:  true,
			Adapter:  "memory",
			Interval: 60,
			TTL:      16 * time.Hour,
		},
		LastCommit: struct {
			Enabled      bool
			TTL          time.Duration `ini:"ITEM_TTL"`
			CommitsCount int64
		}{
			Enabled:      true,
			TTL:          8760 * time.Hour,
			CommitsCount: 1000,
		},
	}
)

// MemcacheMaxTTL represents the maximum memcache TTL
const MemcacheMaxTTL = 30 * 24 * time.Hour

func newCacheService() {
	sec := Cfg.Section("cache")
	if err := sec.MapTo(&CacheService); err != nil {
		log.Fatal("Failed to map Cache settings: %v", err)
	}

	CacheService.Adapter = sec.Key("ADAPTER").In("memory", []string{"memory", "ledis", "redis", "memcache"})
	CacheService.AdapterConfig = sec.Key("ADAPTER_CONFIG").String()
	switch CacheService.Adapter {
	case "memory":
	case "ledis":
	case "redis", "memcache":
		if CacheService.AdapterConfig == "" {
			CacheService.AdapterConfig = sec.Key("HOST").String()
		}
		CacheService.AdapterConfig = strings.Trim(CacheService.AdapterConfig, "\" ")
	case "": // disable cache
		CacheService.Enabled = false
	default:
		log.Fatal("Unknown cache adapter: %s", CacheService.Adapter)
	}

	if CacheService.Enabled {
		log.Info("Cache Service Enabled")
	} else {
		log.Warn("Cache Service Disabled so that captcha disabled too")
		// captcha depends on cache service
		Service.EnableCaptcha = false
	}

	sec = Cfg.Section("cache.last_commit")
	if !CacheService.Enabled {
		CacheService.LastCommit.Enabled = false
	}

	CacheService.LastCommit.CommitsCount = sec.Key("COMMITS_COUNT").MustInt64(1000)

	if CacheService.LastCommit.Enabled {
		log.Info("Last Commit Cache Service Enabled")
	}
}

// TTLSeconds returns the TTLSeconds or unix timestamp for memcache
func (c Cache) TTLSeconds() int64 {
	if c.Adapter == "memcache" && c.TTL > MemcacheMaxTTL {
		return time.Now().Add(c.TTL).Unix()
	}
	return int64(c.TTL.Seconds())
}

// LastCommitCacheTTLSeconds returns the TTLSeconds or unix timestamp for memcache
func LastCommitCacheTTLSeconds() int64 {
	if CacheService.Adapter == "memcache" && CacheService.LastCommit.TTL > MemcacheMaxTTL {
		return time.Now().Add(CacheService.LastCommit.TTL).Unix()
	}
	return int64(CacheService.LastCommit.TTL.Seconds())
}
