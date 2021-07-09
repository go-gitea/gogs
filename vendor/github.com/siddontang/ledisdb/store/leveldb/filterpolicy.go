// +build leveldb

package leveldb

// #cgo LDFLAGS: -lleveldb
// #include <stdlib.h>
// #include "leveldb/c.h"
import "C"

type FilterPolicy struct {
	Policy *C.leveldb_filterpolicy_t
}

func NewBloomFilter(bitsPerKey int) *FilterPolicy {
	policy := C.leveldb_filterpolicy_create_bloom(C.int(bitsPerKey))
	return &FilterPolicy{policy}
}

func (fp *FilterPolicy) Close() {
	C.leveldb_filterpolicy_destroy(fp.Policy)
}
