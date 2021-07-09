package store

import (
	"sync"
	"time"

	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/store/driver"
)

type DB struct {
	db   driver.IDB
	name string

	st *Stat

	cfg *config.Config

	lastCommit time.Time

	m sync.Mutex
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) String() string {
	return db.name
}

func (db *DB) NewIterator() *Iterator {
	db.st.IterNum.Add(1)

	it := new(Iterator)
	it.it = db.db.NewIterator()
	it.st = db.st

	return it
}

func (db *DB) Get(key []byte) ([]byte, error) {
	t := time.Now()
	v, err := db.db.Get(key)
	db.st.statGet(v, err)
	db.st.GetTotalTime.Add(time.Now().Sub(t))
	return v, err
}

func (db *DB) Put(key []byte, value []byte) error {
	db.st.PutNum.Add(1)

	if db.needSyncCommit() {
		return db.db.SyncPut(key, value)

	} else {
		return db.db.Put(key, value)

	}
}

func (db *DB) Delete(key []byte) error {
	db.st.DeleteNum.Add(1)

	if db.needSyncCommit() {
		return db.db.SyncDelete(key)
	} else {
		return db.db.Delete(key)
	}
}

func (db *DB) NewWriteBatch() *WriteBatch {
	db.st.BatchNum.Add(1)
	wb := new(WriteBatch)
	wb.wb = db.db.NewWriteBatch()
	wb.st = db.st
	wb.db = db
	return wb
}

func (db *DB) NewSnapshot() (*Snapshot, error) {
	db.st.SnapshotNum.Add(1)

	var err error
	s := &Snapshot{}
	if s.ISnapshot, err = db.db.NewSnapshot(); err != nil {
		return nil, err
	}
	s.st = db.st

	return s, nil
}

func (db *DB) Compact() error {
	db.st.CompactNum.Add(1)

	t := time.Now()
	err := db.db.Compact()

	db.st.CompactTotalTime.Add(time.Now().Sub(t))

	return err
}

func (db *DB) RangeIterator(min []byte, max []byte, rangeType uint8) *RangeLimitIterator {
	return NewRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{0, -1})
}

func (db *DB) RevRangeIterator(min []byte, max []byte, rangeType uint8) *RangeLimitIterator {
	return NewRevRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{0, -1})
}

//count < 0, unlimit.
//
//offset must >= 0, if < 0, will get nothing.
func (db *DB) RangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *RangeLimitIterator {
	return NewRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{offset, count})
}

//count < 0, unlimit.
//
//offset must >= 0, if < 0, will get nothing.
func (db *DB) RevRangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *RangeLimitIterator {
	return NewRevRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{offset, count})
}

func (db *DB) Stat() *Stat {
	return db.st
}

func (db *DB) needSyncCommit() bool {
	if db.cfg.DBSyncCommit == 0 {
		return false
	} else if db.cfg.DBSyncCommit == 2 {
		return true
	} else {
		n := time.Now()
		need := false
		db.m.Lock()

		if n.Sub(db.lastCommit) > time.Second {
			need = true
		}
		db.lastCommit = n

		db.m.Unlock()
		return need
	}

}

func (db *DB) GetSlice(key []byte) (Slice, error) {
	if d, ok := db.db.(driver.ISliceGeter); ok {
		t := time.Now()
		v, err := d.GetSlice(key)
		db.st.statGet(v, err)
		db.st.GetTotalTime.Add(time.Now().Sub(t))
		return v, err
	} else {
		v, err := db.Get(key)
		if err != nil {
			return nil, err
		} else if v == nil {
			return nil, nil
		} else {
			return driver.GoSlice(v), nil
		}
	}
}
