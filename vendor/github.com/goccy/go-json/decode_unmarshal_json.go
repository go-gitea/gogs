package json

import (
	"unsafe"
)

type unmarshalJSONDecoder struct {
	typ        *rtype
	structName string
	fieldName  string
}

func newUnmarshalJSONDecoder(typ *rtype, structName, fieldName string) *unmarshalJSONDecoder {
	return &unmarshalJSONDecoder{
		typ:        typ,
		structName: structName,
		fieldName:  fieldName,
	}
}

func (d *unmarshalJSONDecoder) annotateError(cursor int64, err error) {
	switch e := err.(type) {
	case *UnmarshalTypeError:
		e.Struct = d.structName
		e.Field = d.fieldName
	case *SyntaxError:
		e.Offset = cursor
	}
}

func (d *unmarshalJSONDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	s.skipWhiteSpace()
	start := s.cursor
	if err := s.skipValue(depth); err != nil {
		return err
	}
	src := s.buf[start:s.cursor]
	dst := make([]byte, len(src))
	copy(dst, src)

	v := *(*interface{})(unsafe.Pointer(&emptyInterface{
		typ: d.typ,
		ptr: p,
	}))
	if err := v.(Unmarshaler).UnmarshalJSON(dst); err != nil {
		d.annotateError(s.cursor, err)
		return err
	}
	return nil
}

func (d *unmarshalJSONDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	cursor = skipWhiteSpace(buf, cursor)
	start := cursor
	end, err := skipValue(buf, cursor, depth)
	if err != nil {
		return 0, err
	}
	src := buf[start:end]
	dst := make([]byte, len(src))
	copy(dst, src)

	v := *(*interface{})(unsafe.Pointer(&emptyInterface{
		typ: d.typ,
		ptr: p,
	}))
	if err := v.(Unmarshaler).UnmarshalJSON(dst); err != nil {
		d.annotateError(cursor, err)
		return 0, err
	}
	return end, nil
}
