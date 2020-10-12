// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nosql

import (
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/syndtr/goleveldb/leveldb"
)

var manager *Manager

// Manager is the nosql connection manager
type Manager struct {
	mutex sync.Mutex

	RedisConnections   map[string]*redisClientHolder
	LevelDBConnections map[string]*levelDBHolder
}

type redisClientHolder struct {
	redis.UniversalClient
	name  []string
	count int64
}

func (r *redisClientHolder) Close() error {
	return manager.CloseRedisClient(r.name[0])
}

type levelDBHolder struct {
	name  []string
	count int64
	db    *leveldb.DB
}

func init() {
	_ = GetManager()
}

// GetManager returns a Manager and initializes one as singleton is there's none yet
func GetManager() *Manager {
	if manager == nil {
		manager = &Manager{
			RedisConnections:   make(map[string]*redisClientHolder),
			LevelDBConnections: make(map[string]*levelDBHolder),
		}
	}
	return manager
}

func valToTimeDuration(vs []string) (result time.Duration) {
	var err error
	for _, v := range vs {
		result, err = time.ParseDuration(v)
		if err != nil {
			var val int
			val, err = strconv.Atoi(v)
			result = time.Duration(val)
		}
		if err == nil {
			return
		}
	}
	return
}
