package json

import (
	"unsafe"
)

type mapDecoder struct {
	mapType      *rtype
	keyType      *rtype
	valueType    *rtype
	keyDecoder   decoder
	valueDecoder decoder
	structName   string
	fieldName    string
}

func newMapDecoder(mapType *rtype, keyType *rtype, keyDec decoder, valueType *rtype, valueDec decoder, structName, fieldName string) *mapDecoder {
	return &mapDecoder{
		mapType:      mapType,
		keyDecoder:   keyDec,
		keyType:      keyType,
		valueType:    valueType,
		valueDecoder: valueDec,
		structName:   structName,
		fieldName:    fieldName,
	}
}

//go:linkname makemap reflect.makemap
func makemap(*rtype, int) unsafe.Pointer

//go:linkname mapassign reflect.mapassign
//go:noescape
func mapassign(t *rtype, m unsafe.Pointer, key, val unsafe.Pointer)

func (d *mapDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	depth++
	if depth > maxDecodeNestingDepth {
		return errExceededMaxDepth(s.char(), s.cursor)
	}

	s.skipWhiteSpace()
	switch s.char() {
	case 'n':
		if err := nullBytes(s); err != nil {
			return err
		}
		**(**unsafe.Pointer)(unsafe.Pointer(&p)) = nil
		return nil
	case '{':
	default:
		return errExpected("{ character for map value", s.totalOffset())
	}
	s.skipWhiteSpace()
	mapValue := *(*unsafe.Pointer)(p)
	if mapValue == nil {
		mapValue = makemap(d.mapType, 0)
	}
	if s.buf[s.cursor+1] == '}' {
		*(*unsafe.Pointer)(p) = mapValue
		s.cursor += 2
		return nil
	}
	for {
		s.cursor++
		k := unsafe_New(d.keyType)
		if err := d.keyDecoder.decodeStream(s, depth, k); err != nil {
			return err
		}
		s.skipWhiteSpace()
		if !s.equalChar(':') {
			return errExpected("colon after object key", s.totalOffset())
		}
		s.cursor++
		v := unsafe_New(d.valueType)
		if err := d.valueDecoder.decodeStream(s, depth, v); err != nil {
			return err
		}
		mapassign(d.mapType, mapValue, k, v)
		s.skipWhiteSpace()
		if s.equalChar('}') {
			**(**unsafe.Pointer)(unsafe.Pointer(&p)) = mapValue
			s.cursor++
			return nil
		}
		if !s.equalChar(',') {
			return errExpected("comma after object value", s.totalOffset())
		}
	}
}

func (d *mapDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	depth++
	if depth > maxDecodeNestingDepth {
		return 0, errExceededMaxDepth(buf[cursor], cursor)
	}

	cursor = skipWhiteSpace(buf, cursor)
	buflen := int64(len(buf))
	if buflen < 2 {
		return 0, errExpected("{} for map", cursor)
	}
	switch buf[cursor] {
	case 'n':
		if err := validateNull(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		**(**unsafe.Pointer)(unsafe.Pointer(&p)) = nil
		return cursor, nil
	case '{':
	default:
		return 0, errExpected("{ character for map value", cursor)
	}
	cursor++
	cursor = skipWhiteSpace(buf, cursor)
	mapValue := *(*unsafe.Pointer)(p)
	if mapValue == nil {
		mapValue = makemap(d.mapType, 0)
	}
	if buf[cursor] == '}' {
		**(**unsafe.Pointer)(unsafe.Pointer(&p)) = mapValue
		cursor++
		return cursor, nil
	}
	for {
		k := unsafe_New(d.keyType)
		keyCursor, err := d.keyDecoder.decode(buf, cursor, depth, k)
		if err != nil {
			return 0, err
		}
		cursor = skipWhiteSpace(buf, keyCursor)
		if buf[cursor] != ':' {
			return 0, errExpected("colon after object key", cursor)
		}
		cursor++
		v := unsafe_New(d.valueType)
		valueCursor, err := d.valueDecoder.decode(buf, cursor, depth, v)
		if err != nil {
			return 0, err
		}
		mapassign(d.mapType, mapValue, k, v)
		cursor = skipWhiteSpace(buf, valueCursor)
		if buf[cursor] == '}' {
			**(**unsafe.Pointer)(unsafe.Pointer(&p)) = mapValue
			cursor++
			return cursor, nil
		}
		if buf[cursor] != ',' {
			return 0, errExpected("comma after object value", cursor)
		}
		cursor++
	}
}
