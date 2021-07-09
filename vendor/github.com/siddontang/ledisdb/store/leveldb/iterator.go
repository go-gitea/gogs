// +build leveldb

package leveldb

// #cgo LDFLAGS: -lleveldb
// #include <stdlib.h>
// #include "leveldb/c.h"
// #include "leveldb_ext.h"
import "C"

import (
	"unsafe"
)

type Iterator struct {
	it      *C.leveldb_iterator_t
	isValid C.uchar
}

func (it *Iterator) Key() []byte {
	var klen C.size_t
	kdata := C.leveldb_iter_key(it.it, &klen)
	if kdata == nil {
		return nil
	}

	return slice(unsafe.Pointer(kdata), int(C.int(klen)))
}

func (it *Iterator) Value() []byte {
	var vlen C.size_t
	vdata := C.leveldb_iter_value(it.it, &vlen)
	if vdata == nil {
		return nil
	}

	return slice(unsafe.Pointer(vdata), int(C.int(vlen)))
}

func (it *Iterator) Close() error {
	if it.it != nil {
		C.leveldb_iter_destroy(it.it)
		it.it = nil
	}
	return nil
}

func (it *Iterator) Valid() bool {
	return ucharToBool(it.isValid)
}

func (it *Iterator) Next() {
	it.isValid = C.leveldb_iter_next_ext(it.it)
}

func (it *Iterator) Prev() {
	it.isValid = C.leveldb_iter_prev_ext(it.it)
}

func (it *Iterator) First() {
	it.isValid = C.leveldb_iter_seek_to_first_ext(it.it)
}

func (it *Iterator) Last() {
	it.isValid = C.leveldb_iter_seek_to_last_ext(it.it)
}

func (it *Iterator) Seek(key []byte) {
	it.isValid = C.leveldb_iter_seek_ext(it.it, (*C.char)(unsafe.Pointer(&key[0])), C.size_t(len(key)))
}
