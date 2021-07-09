package rpl

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/siddontang/go/num"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/store"
)

type GoLevelDBStore struct {
	LogStore

	m  sync.Mutex
	db *store.DB

	cfg *config.Config

	first uint64
	last  uint64

	buf bytes.Buffer
}

func (s *GoLevelDBStore) FirstID() (uint64, error) {
	s.m.Lock()
	id, err := s.firstID()
	s.m.Unlock()

	return id, err
}

func (s *GoLevelDBStore) LastID() (uint64, error) {
	s.m.Lock()
	id, err := s.lastID()
	s.m.Unlock()

	return id, err
}

func (s *GoLevelDBStore) firstID() (uint64, error) {
	if s.first != InvalidLogID {
		return s.first, nil
	}

	it := s.db.NewIterator()
	defer it.Close()

	it.SeekToFirst()

	if it.Valid() {
		s.first = num.BytesToUint64(it.RawKey())
	}

	return s.first, nil
}

func (s *GoLevelDBStore) lastID() (uint64, error) {
	if s.last != InvalidLogID {
		return s.last, nil
	}

	it := s.db.NewIterator()
	defer it.Close()

	it.SeekToLast()

	if it.Valid() {
		s.last = num.BytesToUint64(it.RawKey())
	}

	return s.last, nil
}

func (s *GoLevelDBStore) GetLog(id uint64, log *Log) error {
	v, err := s.db.Get(num.Uint64ToBytes(id))
	if err != nil {
		return err
	} else if v == nil {
		return ErrLogNotFound
	} else {
		return log.Decode(bytes.NewBuffer(v))
	}
}

func (s *GoLevelDBStore) StoreLog(log *Log) error {
	s.m.Lock()
	defer s.m.Unlock()

	last, err := s.lastID()
	if err != nil {
		return err
	}

	s.last = InvalidLogID

	s.buf.Reset()

	if log.ID != last+1 {
		return ErrStoreLogID
	}

	last = log.ID
	key := num.Uint64ToBytes(log.ID)

	if err := log.Encode(&s.buf); err != nil {
		return err
	}

	if err = s.db.Put(key, s.buf.Bytes()); err != nil {
		return err
	}

	s.last = last
	return nil
}

func (s *GoLevelDBStore) PurgeExpired(n int64) error {
	if n <= 0 {
		return fmt.Errorf("invalid expired time %d", n)
	}

	t := uint32(time.Now().Unix() - int64(n))

	s.m.Lock()
	defer s.m.Unlock()

	s.reset()

	it := s.db.NewIterator()
	it.SeekToFirst()

	w := s.db.NewWriteBatch()
	defer w.Rollback()

	l := new(Log)
	for ; it.Valid(); it.Next() {
		v := it.RawValue()

		if err := l.Unmarshal(v); err != nil {
			return err
		} else if l.CreateTime > t {
			break
		} else {
			w.Delete(it.RawKey())
		}
	}

	if err := w.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *GoLevelDBStore) Sync() error {
	//no other way for sync, so ignore here
	return nil
}

func (s *GoLevelDBStore) reset() {
	s.first = InvalidLogID
	s.last = InvalidLogID
}

func (s *GoLevelDBStore) Clear() error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.db != nil {
		s.db.Close()
	}

	s.reset()
	os.RemoveAll(s.cfg.DBPath)

	return s.open()
}

func (s *GoLevelDBStore) Close() error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.db == nil {
		return nil
	}

	err := s.db.Close()
	s.db = nil
	return err
}

func (s *GoLevelDBStore) open() error {
	var err error

	s.first = InvalidLogID
	s.last = InvalidLogID

	s.db, err = store.Open(s.cfg)
	return err
}

func NewGoLevelDBStore(base string, syncLog int) (*GoLevelDBStore, error) {
	cfg := config.NewConfigDefault()
	cfg.DBName = "goleveldb"
	cfg.DBPath = base
	cfg.LevelDB.BlockSize = 16 * 1024 * 1024
	cfg.LevelDB.CacheSize = 64 * 1024 * 1024
	cfg.LevelDB.WriteBufferSize = 64 * 1024 * 1024
	cfg.LevelDB.Compression = false
	cfg.DBSyncCommit = syncLog

	s := new(GoLevelDBStore)
	s.cfg = cfg

	if err := s.open(); err != nil {
		return nil, err
	}

	return s, nil
}
