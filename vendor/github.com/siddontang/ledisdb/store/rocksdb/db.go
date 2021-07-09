// +build rocksdb

// Package rocksdb is a wrapper for c++ rocksdb
package rocksdb

/*
#cgo LDFLAGS: -lrocksdb
#include <rocksdb/c.h>
#include <stdlib.h>
#include "rocksdb_ext.h"
*/
import "C"

import (
	"os"
	"runtime"
	"unsafe"

	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/store/driver"
)

const defaultFilterBits int = 10

type Store struct {
}

func (s Store) String() string {
	return DBName
}

func (s Store) Open(path string, cfg *config.Config) (driver.IDB, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	db := new(DB)
	db.path = path
	db.cfg = &cfg.RocksDB

	if err := db.open(); err != nil {
		return nil, err
	}

	return db, nil
}

func (s Store) Repair(path string, cfg *config.Config) error {
	db := new(DB)
	db.path = path
	db.cfg = &cfg.RocksDB

	err := db.open()
	defer db.Close()

	//open ok, do not need repair
	if err == nil {
		return nil
	}

	var errStr *C.char
	ldbname := C.CString(path)
	defer C.free(unsafe.Pointer(ldbname))

	C.rocksdb_repair_db(db.opts.Opt, ldbname, &errStr)
	if errStr != nil {
		return saveError(errStr)
	}
	return nil
}

type DB struct {
	path string

	cfg *config.RocksDBConfig

	db *C.rocksdb_t

	env *Env

	opts      *Options
	blockOpts *BlockBasedTableOptions

	//for default read and write options
	readOpts     *ReadOptions
	writeOpts    *WriteOptions
	iteratorOpts *ReadOptions

	syncOpts *WriteOptions

	cache *Cache

	filter *FilterPolicy
}

func (db *DB) open() error {
	db.initOptions(db.cfg)

	var errStr *C.char
	ldbname := C.CString(db.path)
	defer C.free(unsafe.Pointer(ldbname))

	db.db = C.rocksdb_open(db.opts.Opt, ldbname, &errStr)
	if errStr != nil {
		db.db = nil
		return saveError(errStr)
	}
	return nil
}

func (db *DB) initOptions(cfg *config.RocksDBConfig) {
	opts := NewOptions()
	blockOpts := NewBlockBasedTableOptions()

	opts.SetCreateIfMissing(true)

	db.env = NewDefaultEnv()
	db.env.SetBackgroundThreads(cfg.BackgroundThreads)
	db.env.SetHighPriorityBackgroundThreads(cfg.HighPriorityBackgroundThreads)
	opts.SetEnv(db.env)

	db.cache = NewLRUCache(cfg.CacheSize)
	blockOpts.SetCache(db.cache)

	//we must use bloomfilter
	db.filter = NewBloomFilter(defaultFilterBits)
	blockOpts.SetFilterPolicy(db.filter)
	blockOpts.SetBlockSize(cfg.BlockSize)
	opts.SetBlockBasedTableFactory(blockOpts)

	opts.SetCompression(CompressionOpt(cfg.Compression))
	opts.SetWriteBufferSize(cfg.WriteBufferSize)
	opts.SetMaxOpenFiles(cfg.MaxOpenFiles)
	opts.SetMaxBackgroundCompactions(cfg.MaxBackgroundCompactions)
	opts.SetMaxBackgroundFlushes(cfg.MaxBackgroundFlushes)
	opts.SetLevel0FileNumCompactionTrigger(cfg.Level0FileNumCompactionTrigger)
	opts.SetLevel0SlowdownWritesTrigger(cfg.Level0SlowdownWritesTrigger)
	opts.SetLevel0StopWritesTrigger(cfg.Level0StopWritesTrigger)
	opts.SetTargetFileSizeBase(cfg.TargetFileSizeBase)
	opts.SetTargetFileSizeMultiplier(cfg.TargetFileSizeMultiplier)
	opts.SetMaxBytesForLevelBase(cfg.MaxBytesForLevelBase)
	opts.SetMaxBytesForLevelMultiplier(cfg.MaxBytesForLevelMultiplier)
	opts.SetMinWriteBufferNumberToMerge(cfg.MinWriteBufferNumberToMerge)
	opts.DisableAutoCompactions(cfg.DisableAutoCompactions)
	opts.EnableStatistics(cfg.EnableStatistics)
	opts.UseFsync(cfg.UseFsync)
	opts.SetStatsDumpPeriodSec(cfg.StatsDumpPeriodSec)
	opts.SetMaxManifestFileSize(cfg.MaxManifestFileSize)

	db.opts = opts
	db.blockOpts = blockOpts

	db.readOpts = NewReadOptions()
	db.writeOpts = NewWriteOptions()
	db.writeOpts.DisableWAL(cfg.DisableWAL)

	db.syncOpts = NewWriteOptions()
	db.syncOpts.SetSync(true)
	db.syncOpts.DisableWAL(cfg.DisableWAL)

	db.iteratorOpts = NewReadOptions()
	db.iteratorOpts.SetFillCache(false)
}

func (db *DB) Close() error {
	if db.db != nil {
		C.rocksdb_close(db.db)
		db.db = nil
	}

	if db.filter != nil {
		db.filter.Close()
	}

	if db.cache != nil {
		db.cache.Close()
	}

	if db.env != nil {
		db.env.Close()
	}

	//db.blockOpts.Close()

	db.opts.Close()

	db.readOpts.Close()
	db.writeOpts.Close()
	db.iteratorOpts.Close()

	return nil
}

func (db *DB) Put(key, value []byte) error {
	return db.put(db.writeOpts, key, value)
}

func (db *DB) Get(key []byte) ([]byte, error) {
	return db.get(db.readOpts, key)
}

func (db *DB) Delete(key []byte) error {
	return db.delete(db.writeOpts, key)
}

func (db *DB) SyncPut(key []byte, value []byte) error {
	return db.put(db.syncOpts, key, value)
}

func (db *DB) SyncDelete(key []byte) error {
	return db.delete(db.syncOpts, key)
}

func (db *DB) NewWriteBatch() driver.IWriteBatch {
	wb := &WriteBatch{
		db:     db,
		wbatch: C.rocksdb_writebatch_create(),
	}

	runtime.SetFinalizer(wb, func(w *WriteBatch) {
		w.Close()
	})

	return wb
}

func (db *DB) NewIterator() driver.IIterator {
	it := new(Iterator)

	it.it = C.rocksdb_create_iterator(db.db, db.iteratorOpts.Opt)

	return it
}

func (db *DB) NewSnapshot() (driver.ISnapshot, error) {
	snap := &Snapshot{
		db:           db,
		snap:         C.rocksdb_create_snapshot(db.db),
		readOpts:     NewReadOptions(),
		iteratorOpts: NewReadOptions(),
	}
	snap.readOpts.SetSnapshot(snap)
	snap.iteratorOpts.SetSnapshot(snap)
	snap.iteratorOpts.SetFillCache(false)

	return snap, nil
}

func (db *DB) put(wo *WriteOptions, key, value []byte) error {
	var errStr *C.char
	var k, v *C.char
	if len(key) != 0 {
		k = (*C.char)(unsafe.Pointer(&key[0]))
	}
	if len(value) != 0 {
		v = (*C.char)(unsafe.Pointer(&value[0]))
	}

	lenk := len(key)
	lenv := len(value)
	C.rocksdb_put(
		db.db, wo.Opt, k, C.size_t(lenk), v, C.size_t(lenv), &errStr)

	if errStr != nil {
		return saveError(errStr)
	}
	return nil
}

func (db *DB) get(ro *ReadOptions, key []byte) ([]byte, error) {
	var errStr *C.char
	var vallen C.size_t
	var k *C.char
	if len(key) != 0 {
		k = (*C.char)(unsafe.Pointer(&key[0]))
	}

	value := C.rocksdb_get(
		db.db, ro.Opt, k, C.size_t(len(key)), &vallen, &errStr)

	if errStr != nil {
		return nil, saveError(errStr)
	}

	if value == nil {
		return nil, nil
	}

	defer C.free(unsafe.Pointer(value))
	return C.GoBytes(unsafe.Pointer(value), C.int(vallen)), nil
}

func (db *DB) getSlice(ro *ReadOptions, key []byte) (driver.ISlice, error) {
	var errStr *C.char
	var vallen C.size_t
	var k *C.char
	if len(key) != 0 {
		k = (*C.char)(unsafe.Pointer(&key[0]))
	}

	value := C.rocksdb_get(
		db.db, ro.Opt, k, C.size_t(len(key)), &vallen, &errStr)

	if errStr != nil {
		return nil, saveError(errStr)
	}

	if value == nil {
		return nil, nil
	}

	return NewCSlice(unsafe.Pointer(value), int(vallen)), nil
}

func (db *DB) delete(wo *WriteOptions, key []byte) error {
	var errStr *C.char
	var k *C.char
	if len(key) != 0 {
		k = (*C.char)(unsafe.Pointer(&key[0]))
	}

	C.rocksdb_delete(
		db.db, wo.Opt, k, C.size_t(len(key)), &errStr)

	if errStr != nil {
		return saveError(errStr)
	}
	return nil
}

func (db *DB) Compact() error {
	C.rocksdb_compact_range(db.db, nil, 0, nil, 0)
	return nil
}

func (db *DB) GetSlice(key []byte) (driver.ISlice, error) {
	return db.getSlice(db.readOpts, key)
}

func init() {
	driver.Register(Store{})
}
