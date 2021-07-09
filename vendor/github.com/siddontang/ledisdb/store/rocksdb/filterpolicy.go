// +build rocksdb

package rocksdb

// #cgo LDFLAGS: -lrocksdb
// #include <stdlib.h>
// #include "rocksdb/c.h"
import "C"

type FilterPolicy struct {
	Policy *C.rocksdb_filterpolicy_t
}

func NewBloomFilter(bitsPerKey int) *FilterPolicy {
	policy := C.rocksdb_filterpolicy_create_bloom(C.int(bitsPerKey))
	return &FilterPolicy{policy}
}

func (fp *FilterPolicy) Close() {
	C.rocksdb_filterpolicy_destroy(fp.Policy)
}
