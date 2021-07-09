// +build rocksdb

package rocksdb

// #include <stdlib.h>
// #include "rocksdb/c.h"
import "C"

import (
	"fmt"
	"reflect"
	"unsafe"
)

func boolToUchar(b bool) C.uchar {
	uc := C.uchar(0)
	if b {
		uc = C.uchar(1)
	}
	return uc
}

func ucharToBool(uc C.uchar) bool {
	if uc == C.uchar(0) {
		return false
	}
	return true
}

func boolToInt(b bool) C.int {
	uc := C.int(0)
	if b {
		uc = C.int(1)
	}
	return uc
}

func saveError(errStr *C.char) error {
	if errStr != nil {
		gs := C.GoString(errStr)
		C.free(unsafe.Pointer(errStr))
		return fmt.Errorf(gs)
	}
	return nil
}

func slice(p unsafe.Pointer, n int) []byte {
	var b []byte
	pbyte := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pbyte.Data = uintptr(p)
	pbyte.Len = n
	pbyte.Cap = n
	return b
}
