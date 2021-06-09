// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package session

import (
	"fmt"
	"sync"

	"gitea.com/go-chi/session"
	couchbase "gitea.com/go-chi/session/couchbase"
	memcache "gitea.com/go-chi/session/memcache"
	mysql "gitea.com/go-chi/session/mysql"
	postgres "gitea.com/go-chi/session/postgres"
	"github.com/goccy/go-json"
)

// VirtualSessionProvider represents a shadowed session provider implementation.
type VirtualSessionProvider struct {
	lock     sync.RWMutex
	provider session.Provider
}

// Init initializes the cookie session provider with given root path.
func (o *VirtualSessionProvider) Init(gclifetime int64, config string) error {
	var opts session.Options
	if err := json.Unmarshal([]byte(config), &opts); err != nil {
		return err
	}
	// Note that these options are unprepared so we can't just use NewManager here.
	// Nor can we access the provider map in session.
	// So we will just have to do this by hand.
	// This is only slightly more wrong than modules/setting/session.go:23
	switch opts.Provider {
	case "memory":
		o.provider = &session.MemProvider{}
	case "file":
		o.provider = &session.FileProvider{}
	case "redis":
		o.provider = &RedisProvider{}
	case "db":
		o.provider = &DBProvider{}
	case "mysql":
		o.provider = &mysql.MysqlProvider{}
	case "postgres":
		o.provider = &postgres.PostgresProvider{}
	case "couchbase":
		o.provider = &couchbase.CouchbaseProvider{}
	case "memcache":
		o.provider = &memcache.MemcacheProvider{}
	default:
		return fmt.Errorf("VirtualSessionProvider: Unknown Provider: %s", opts.Provider)
	}
	return o.provider.Init(gclifetime, opts.ProviderConfig)
}

// Read returns raw session store by session ID.
func (o *VirtualSessionProvider) Read(sid string) (session.RawStore, error) {
	o.lock.RLock()
	defer o.lock.RUnlock()
	if o.provider.Exist(sid) {
		return o.provider.Read(sid)
	}
	kv := make(map[interface{}]interface{})
	kv["_old_uid"] = "0"
	return NewVirtualStore(o, sid, kv), nil
}

// Exist returns true if session with given ID exists.
func (o *VirtualSessionProvider) Exist(sid string) bool {
	return true
}

// Destroy deletes a session by session ID.
func (o *VirtualSessionProvider) Destroy(sid string) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	return o.provider.Destroy(sid)
}

// Regenerate regenerates a session store from old session ID to new one.
func (o *VirtualSessionProvider) Regenerate(oldsid, sid string) (session.RawStore, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	return o.provider.Regenerate(oldsid, sid)
}

// Count counts and returns number of sessions.
func (o *VirtualSessionProvider) Count() int {
	o.lock.RLock()
	defer o.lock.RUnlock()
	return o.provider.Count()
}

// GC calls GC to clean expired sessions.
func (o *VirtualSessionProvider) GC() {
	o.provider.GC()
}

func init() {
	session.Register("VirtualSession", &VirtualSessionProvider{})
}

// VirtualStore represents a virtual session store implementation.
type VirtualStore struct {
	p        *VirtualSessionProvider
	sid      string
	lock     sync.RWMutex
	data     map[interface{}]interface{}
	released bool
}

// NewVirtualStore creates and returns a virtual session store.
func NewVirtualStore(p *VirtualSessionProvider, sid string, kv map[interface{}]interface{}) *VirtualStore {
	return &VirtualStore{
		p:    p,
		sid:  sid,
		data: kv,
	}
}

// Set sets value to given key in session.
func (s *VirtualStore) Set(key, val interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data[key] = val
	return nil
}

// Get gets value by given key in session.
func (s *VirtualStore) Get(key interface{}) interface{} {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.data[key]
}

// Delete delete a key from session.
func (s *VirtualStore) Delete(key interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.data, key)
	return nil
}

// ID returns current session ID.
func (s *VirtualStore) ID() string {
	return s.sid
}

// Release releases resource and save data to provider.
func (s *VirtualStore) Release() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	// Now need to lock the provider
	s.p.lock.Lock()
	defer s.p.lock.Unlock()
	if oldUID, ok := s.data["_old_uid"]; (ok && (oldUID != "0" || len(s.data) > 1)) || (!ok && len(s.data) > 0) {
		// Now ensure that we don't exist!
		realProvider := s.p.provider

		if !s.released && realProvider.Exist(s.sid) {
			// This is an error!
			return fmt.Errorf("new sid '%s' already exists", s.sid)
		}
		realStore, err := realProvider.Read(s.sid)
		if err != nil {
			return err
		}
		if err := realStore.Flush(); err != nil {
			return err
		}
		for key, value := range s.data {
			if err := realStore.Set(key, value); err != nil {
				return err
			}
		}
		err = realStore.Release()
		if err == nil {
			s.released = true
		}
		return err
	}
	return nil
}

// Flush deletes all session data.
func (s *VirtualStore) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data = make(map[interface{}]interface{})
	return nil
}
