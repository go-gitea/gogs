// Copyright 2014 The Macaron Authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package cache

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/ledis"
	"github.com/unknwon/com"
	"gopkg.in/ini.v1"

	"gitea.com/go-chi/cache"
)

var defaultHSetName = []byte("MacaronCache")

// LedisCacher represents a ledis cache adapter implementation.
type LedisCacher struct {
	c        *ledis.DB
	interval int
}

// Put puts value into cache with key and expire time.
// If expired is 0, it lives forever.
func (c *LedisCacher) Put(key string, val interface{}, expire int64) (err error) {
	if expire == 0 {
		if err = c.c.Set([]byte(key), []byte(com.ToStr(val))); err != nil {
			return err
		}
		_, err = c.c.HSet([]byte(key), defaultHSetName, []byte("0"))
		return err
	}

	if err = c.c.SetEX([]byte(key), expire, []byte(com.ToStr(val))); err != nil {
		return err
	}
	_, err = c.c.HSet([]byte(key), defaultHSetName, []byte(com.ToStr(time.Now().Add(time.Duration(expire)*time.Second).Unix())))
	return err
}

// Get gets cached value by given key.
func (c *LedisCacher) Get(key string) interface{} {
	val, err := c.c.Get([]byte(key))
	if err != nil || len(val) == 0 {
		return nil
	}
	return string(val)
}

// Delete deletes cached value by given key.
func (c *LedisCacher) Delete(key string) (err error) {
	if _, err = c.c.Del([]byte(key)); err != nil {
		return err
	}
	_, err = c.c.HDel(defaultHSetName, []byte(key))
	return err
}

// Incr increases cached int-type value by given key as a counter.
func (c *LedisCacher) Incr(key string) error {
	if !c.IsExist(key) {
		return fmt.Errorf("key '%s' not exist", key)
	}
	_, err := c.c.Incr([]byte(key))
	return err
}

// Decr decreases cached int-type value by given key as a counter.
func (c *LedisCacher) Decr(key string) error {
	if !c.IsExist(key) {
		return fmt.Errorf("key '%s' not exist", key)
	}
	_, err := c.c.Decr([]byte(key))
	return err
}

// IsExist returns true if cached value exists.
func (c *LedisCacher) IsExist(key string) bool {
	count, err := c.c.Exists([]byte(key))
	if err == nil && count > 0 {
		return true
	}
	c.c.HDel(defaultHSetName, []byte(key))
	return false
}

// Flush deletes all cached data.
func (c *LedisCacher) Flush() error {
	// FIXME: there must be something wrong, shouldn't use this one.
	_, err := c.c.FlushAll()
	return err

	// keys, err := c.c.HKeys(defaultHSetName)
	// if err != nil {
	// 	return err
	// }
	// if _, err = c.c.Del(keys...); err != nil {
	// 	return err
	// }
	// _, err = c.c.Del(defaultHSetName)
	// return err
}

func (c *LedisCacher) startGC() {
	if c.interval < 1 {
		return
	}

	kvs, err := c.c.HGetAll(defaultHSetName)
	if err != nil {
		log.Printf("cache/redis: error garbage collecting(get): %v", err)
		return
	}

	now := time.Now().Unix()
	for _, v := range kvs {
		expire := com.StrTo(v.Value).MustInt64()
		if expire == 0 || now < expire {
			continue
		}

		if err = c.Delete(string(v.Field)); err != nil {
			log.Printf("cache/redis: error garbage collecting(delete): %v", err)
			continue
		}
	}

	time.AfterFunc(time.Duration(c.interval)*time.Second, func() { c.startGC() })
}

// StartAndGC starts GC routine based on config string settings.
// AdapterConfig: data_dir=./app.db,db=0
func (c *LedisCacher) StartAndGC(opts cache.Options) error {
	c.interval = opts.Interval

	cfg, err := ini.Load([]byte(strings.Replace(opts.AdapterConfig, ",", "\n", -1)))
	if err != nil {
		return err
	}

	db := 0
	opt := new(config.Config)
	for k, v := range cfg.Section("").KeysHash() {
		switch k {
		case "data_dir":
			opt.DataDir = v
		case "db":
			db = com.StrTo(v).MustInt()
		default:
			return fmt.Errorf("session/ledis: unsupported option '%s'", k)
		}
	}

	l, err := ledis.Open(opt)
	if err != nil {
		return fmt.Errorf("session/ledis: error opening db: %v", err)
	}
	c.c, err = l.Select(db)
	if err != nil {
		return err
	}

	go c.startGC()
	return nil
}

func init() {
	cache.Register("ledis", &LedisCacher{})
}
