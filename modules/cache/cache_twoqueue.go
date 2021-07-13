// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import (
	"strconv"
	"sync"
	"time"

	mc "gitea.com/go-chi/cache"
	lru "github.com/hashicorp/golang-lru"
	jsoniter "github.com/json-iterator/go"
)

// TwoQueueCache represents a LRU 2Q cache adapter implementation
type TwoQueueCache struct {
	lock     sync.Mutex
	cache    *lru.TwoQueueCache
	interval int
}

// TwoQueueCacheConfig describes the configuration for TwoQueueCache
type TwoQueueCacheConfig struct {
	Size        int     `ini:"SIZE" json:"size"`
	RecentRatio float64 `ini:"RECENT_RATIO" json:"recent_ratio"`
	GhostRatio  float64 `ini:"GHOST_RATIO" json:"ghost_ratio"`
}

// MemoryItem represents a memory cache item.
type MemoryItem struct {
	Val     interface{}
	Created int64
	Timeout int64
}

func (item *MemoryItem) hasExpired() bool {
	return item.Timeout > 0 &&
		(time.Now().Unix()-item.Created) >= item.Timeout
}

var _ mc.Cache = &TwoQueueCache{}

// Put puts value into cache with key and expire time.
func (c *TwoQueueCache) Put(key string, val interface{}, timeout int64) error {
	item := &MemoryItem{
		Val:     val,
		Created: time.Now().Unix(),
		Timeout: timeout,
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache.Add(key, item)
	return nil
}

// Get gets cached value by given key.
func (c *TwoQueueCache) Get(key string) interface{} {
	c.lock.Lock()
	defer c.lock.Unlock()
	cached, ok := c.cache.Get(key)
	if !ok {
		return nil
	}
	item, ok := cached.(*MemoryItem)

	if !ok || item.hasExpired() {
		c.cache.Remove(key)
		return nil
	}

	return item.Val
}

// Delete deletes cached value by given key.
func (c *TwoQueueCache) Delete(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache.Remove(key)
	return nil
}

// Incr increases cached int-type value by given key as a counter.
func (c *TwoQueueCache) Incr(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	cached, ok := c.cache.Get(key)
	if !ok {
		return nil
	}
	item, ok := cached.(*MemoryItem)

	if !ok || item.hasExpired() {
		c.cache.Remove(key)
		return nil
	}

	var err error
	item.Val, err = mc.Incr(item.Val)
	return err
}

// Decr decreases cached int-type value by given key as a counter.
func (c *TwoQueueCache) Decr(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	cached, ok := c.cache.Get(key)
	if !ok {
		return nil
	}
	item, ok := cached.(*MemoryItem)

	if !ok || item.hasExpired() {
		c.cache.Remove(key)
		return nil
	}

	var err error
	item.Val, err = mc.Decr(item.Val)
	return err
}

// IsExist returns true if cached value exists.
func (c *TwoQueueCache) IsExist(key string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	cached, ok := c.cache.Peek(key)
	if !ok {
		return false
	}
	item, ok := cached.(*MemoryItem)
	if !ok || item.hasExpired() {
		c.cache.Remove(key)
		return false
	}

	return true
}

// Flush deletes all cached data.
func (c *TwoQueueCache) Flush() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache.Purge()
	return nil
}

func (c *TwoQueueCache) checkAndInvalidate(key interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cached, ok := c.cache.Peek(key)
	if !ok {
		return
	}
	item, ok := cached.(*MemoryItem)
	if !ok || item.hasExpired() {
		c.cache.Remove(item)
	}
}

func (c *TwoQueueCache) startGC() {
	if c.interval < 0 {
		return
	}
	for _, key := range c.cache.Keys() {
		c.checkAndInvalidate(key)
	}
	time.AfterFunc(time.Duration(c.interval)*time.Second, c.startGC)
}

// StartAndGC starts GC routine based on config string settings.
func (c *TwoQueueCache) StartAndGC(opts mc.Options) error {
	var err error
	size := 50000
	if opts.AdapterConfig != "" {
		size, err = strconv.Atoi(opts.AdapterConfig)
	}
	if err != nil {
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		if !json.Valid([]byte(opts.AdapterConfig)) {
			return err
		}

		cfg := &TwoQueueCacheConfig{
			Size:        50000,
			RecentRatio: lru.Default2QRecentRatio,
			GhostRatio:  lru.Default2QGhostEntries,
		}
		_ = json.Unmarshal([]byte(opts.AdapterConfig), cfg)
		c.cache, err = lru.New2QParams(cfg.Size, cfg.RecentRatio, cfg.GhostRatio)
	} else {
		c.cache, err = lru.New2Q(size)
	}
	c.interval = opts.Interval
	if c.interval > 0 {
		go c.startGC()
	}
	return err
}

func init() {
	mc.Register("twoqueue", &TwoQueueCache{})
}
