// +build rocksdb

package rocksdb

// #cgo LDFLAGS: -lrocksdb
// #include <stdint.h>
// #include "rocksdb/c.h"
import "C"

type Cache struct {
	Cache *C.rocksdb_cache_t
}

func NewLRUCache(capacity int) *Cache {
	return &Cache{C.rocksdb_cache_create_lru(C.size_t(capacity))}
}

func (c *Cache) Close() {
	C.rocksdb_cache_destroy(c.Cache)
}
