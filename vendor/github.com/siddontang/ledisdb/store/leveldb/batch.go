// +build leveldb

package leveldb

// #cgo LDFLAGS: -lleveldb
// #include "leveldb/c.h"
// #include "leveldb_ext.h"
import "C"

import (
	"unsafe"

	"github.com/syndtr/goleveldb/leveldb"
)

type WriteBatch struct {
	db     *DB
	wbatch *C.leveldb_writebatch_t
}

func newWriteBatch(db *DB) *WriteBatch {
	w := new(WriteBatch)
	w.db = db
	w.wbatch = C.leveldb_writebatch_create()

	return w
}

func (w *WriteBatch) Close() {
	if w.wbatch != nil {
		C.leveldb_writebatch_destroy(w.wbatch)
		w.wbatch = nil
	}
}

func (w *WriteBatch) Put(key, value []byte) {
	var k, v *C.char
	if len(key) != 0 {
		k = (*C.char)(unsafe.Pointer(&key[0]))
	}
	if len(value) != 0 {
		v = (*C.char)(unsafe.Pointer(&value[0]))
	}

	lenk := len(key)
	lenv := len(value)

	C.leveldb_writebatch_put(w.wbatch, k, C.size_t(lenk), v, C.size_t(lenv))
}

func (w *WriteBatch) Delete(key []byte) {
	C.leveldb_writebatch_delete(w.wbatch,
		(*C.char)(unsafe.Pointer(&key[0])), C.size_t(len(key)))
}

func (w *WriteBatch) Commit() error {
	return w.commit(w.db.writeOpts)
}

func (w *WriteBatch) SyncCommit() error {
	return w.commit(w.db.syncOpts)
}

func (w *WriteBatch) Rollback() error {
	C.leveldb_writebatch_clear(w.wbatch)

	return nil
}

func (w *WriteBatch) commit(wb *WriteOptions) error {
	var errStr *C.char
	C.leveldb_write(w.db.db, wb.Opt, w.wbatch, &errStr)
	if errStr != nil {
		return saveError(errStr)
	}
	return nil
}

//export leveldb_writebatch_iterate_put
func leveldb_writebatch_iterate_put(p unsafe.Pointer, k *C.char, klen C.size_t, v *C.char, vlen C.size_t) {
	b := (*leveldb.Batch)(p)
	key := slice(unsafe.Pointer(k), int(klen))
	value := slice(unsafe.Pointer(v), int(vlen))
	b.Put(key, value)
}

//export leveldb_writebatch_iterate_delete
func leveldb_writebatch_iterate_delete(p unsafe.Pointer, k *C.char, klen C.size_t) {
	b := (*leveldb.Batch)(p)
	key := slice(unsafe.Pointer(k), int(klen))
	b.Delete(key)
}

func (w *WriteBatch) Data() []byte {
	gbatch := leveldb.Batch{}
	C.leveldb_writebatch_iterate_ext(w.wbatch,
		unsafe.Pointer(&gbatch))
	return gbatch.Dump()
}
