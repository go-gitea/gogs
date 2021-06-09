package json

import (
	"unsafe"
)

type anonymousFieldDecoder struct {
	structType *rtype
	offset     uintptr
	dec        decoder
}

func newAnonymousFieldDecoder(structType *rtype, offset uintptr, dec decoder) *anonymousFieldDecoder {
	return &anonymousFieldDecoder{
		structType: structType,
		offset:     offset,
		dec:        dec,
	}
}

func (d *anonymousFieldDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	if *(*unsafe.Pointer)(p) == nil {
		*(*unsafe.Pointer)(p) = unsafe_New(d.structType)
	}
	p = *(*unsafe.Pointer)(p)
	return d.dec.decodeStream(s, depth, unsafe.Pointer(uintptr(p)+d.offset))
}

func (d *anonymousFieldDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	if *(*unsafe.Pointer)(p) == nil {
		*(*unsafe.Pointer)(p) = unsafe_New(d.structType)
	}
	p = *(*unsafe.Pointer)(p)
	return d.dec.decode(buf, cursor, depth, unsafe.Pointer(uintptr(p)+d.offset))
}
