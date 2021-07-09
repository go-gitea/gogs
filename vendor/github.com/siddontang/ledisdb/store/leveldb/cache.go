// +build leveldb

package leveldb

// #cgo LDFLAGS: -lleveldb
// #include <stdint.h>
// #include "leveldb/c.h"
import "C"

type Cache struct {
	Cache *C.leveldb_cache_t
}

func NewLRUCache(capacity int) *Cache {
	return &Cache{C.leveldb_cache_create_lru(C.size_t(capacity))}
}

func (c *Cache) Close() {
	C.leveldb_cache_destroy(c.Cache)
}
