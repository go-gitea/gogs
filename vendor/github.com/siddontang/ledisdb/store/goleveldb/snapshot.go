package goleveldb

import (
	"github.com/siddontang/ledisdb/store/driver"
	"github.com/syndtr/goleveldb/leveldb"
)

type Snapshot struct {
	db  *DB
	snp *leveldb.Snapshot
}

func (s *Snapshot) Get(key []byte) ([]byte, error) {
	return s.snp.Get(key, s.db.iteratorOpts)
}

func (s *Snapshot) NewIterator() driver.IIterator {
	it := &Iterator{
		s.snp.NewIterator(nil, s.db.iteratorOpts),
	}
	return it
}

func (s *Snapshot) Close() {
	s.snp.Release()
}
