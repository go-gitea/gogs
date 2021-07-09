// +build rocksdb

package rocksdb

// #cgo LDFLAGS: -lrocksdb
// #include "rocksdb/c.h"
import "C"

import (
	"github.com/siddontang/ledisdb/store/driver"
)

type Snapshot struct {
	db           *DB
	snap         *C.rocksdb_snapshot_t
	readOpts     *ReadOptions
	iteratorOpts *ReadOptions
}

func (s *Snapshot) Get(key []byte) ([]byte, error) {
	return s.db.get(s.readOpts, key)
}

func (s *Snapshot) GetSlice(key []byte) (driver.ISlice, error) {
	return s.db.getSlice(s.readOpts, key)
}

func (s *Snapshot) NewIterator() driver.IIterator {
	it := new(Iterator)
	it.it = C.rocksdb_create_iterator(s.db.db, s.db.iteratorOpts.Opt)
	return it

}

func (s *Snapshot) Close() {
	C.rocksdb_release_snapshot(s.db.db, s.snap)
	s.iteratorOpts.Close()
	s.readOpts.Close()
}
