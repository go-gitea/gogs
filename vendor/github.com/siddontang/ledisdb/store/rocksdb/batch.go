// +build rocksdb

package rocksdb

// #cgo LDFLAGS: -lrocksdb
// #include "rocksdb/c.h"
// #include "rocksdb_ext.h"
import "C"

import (
	"unsafe"
)

type WriteBatch struct {
	db       *DB
	wbatch   *C.rocksdb_writebatch_t
	commitOk bool
}

func (w *WriteBatch) Close() {
	if w.wbatch != nil {
		C.rocksdb_writebatch_destroy(w.wbatch)
		w.wbatch = nil
	}
}

func (w *WriteBatch) Put(key, value []byte) {
	w.commitOk = false

	var k, v *C.char
	if len(key) != 0 {
		k = (*C.char)(unsafe.Pointer(&key[0]))
	}
	if len(value) != 0 {
		v = (*C.char)(unsafe.Pointer(&value[0]))
	}

	lenk := len(key)
	lenv := len(value)

	C.rocksdb_writebatch_put(w.wbatch, k, C.size_t(lenk), v, C.size_t(lenv))
}

func (w *WriteBatch) Delete(key []byte) {
	w.commitOk = false

	C.rocksdb_writebatch_delete(w.wbatch,
		(*C.char)(unsafe.Pointer(&key[0])), C.size_t(len(key)))
}

func (w *WriteBatch) Commit() error {
	return w.commit(w.db.writeOpts)
}

func (w *WriteBatch) SyncCommit() error {
	return w.commit(w.db.syncOpts)
}

func (w *WriteBatch) Rollback() error {
	if !w.commitOk {
		C.rocksdb_writebatch_clear(w.wbatch)
	}
	return nil
}

func (w *WriteBatch) commit(wb *WriteOptions) error {
	w.commitOk = true

	var errStr *C.char
	C.rocksdb_write_ext(w.db.db, wb.Opt, w.wbatch, &errStr)
	if errStr != nil {
		w.commitOk = false
		return saveError(errStr)
	}
	return nil
}

func (w *WriteBatch) Data() []byte {
	var vallen C.size_t
	value := C.rocksdb_writebatch_data(w.wbatch, &vallen)

	return slice(unsafe.Pointer(value), int(vallen))
}
