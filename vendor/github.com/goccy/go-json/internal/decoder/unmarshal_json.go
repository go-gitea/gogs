package decoder

import (
	"encoding/json"
	"unsafe"

	"github.com/goccy/go-json/internal/errors"
	"github.com/goccy/go-json/internal/runtime"
)

type unmarshalJSONDecoder struct {
	typ        *runtime.Type
	structName string
	fieldName  string
}

func newUnmarshalJSONDecoder(typ *runtime.Type, structName, fieldName string) *unmarshalJSONDecoder {
	return &unmarshalJSONDecoder{
		typ:        typ,
		structName: structName,
		fieldName:  fieldName,
	}
}

func (d *unmarshalJSONDecoder) annotateError(cursor int64, err error) {
	switch e := err.(type) {
	case *errors.UnmarshalTypeError:
		e.Struct = d.structName
		e.Field = d.fieldName
	case *errors.SyntaxError:
		e.Offset = cursor
	}
}

func (d *unmarshalJSONDecoder) DecodeStream(s *Stream, depth int64, p unsafe.Pointer) error {
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
	if (s.Option.Flags & ContextOption) != 0 {
		if err := v.(unmarshalerContext).UnmarshalJSON(s.Option.Context, dst); err != nil {
			d.annotateError(s.cursor, err)
			return err
		}
	} else {
		if err := v.(json.Unmarshaler).UnmarshalJSON(dst); err != nil {
			d.annotateError(s.cursor, err)
			return err
		}
	}
	return nil
}

func (d *unmarshalJSONDecoder) Decode(ctx *RuntimeContext, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	buf := ctx.Buf
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
	if (ctx.Option.Flags & ContextOption) != 0 {
		if err := v.(unmarshalerContext).UnmarshalJSON(ctx.Option.Context, dst); err != nil {
			d.annotateError(cursor, err)
			return 0, err
		}
	} else {
		if err := v.(json.Unmarshaler).UnmarshalJSON(dst); err != nil {
			d.annotateError(cursor, err)
			return 0, err
		}
	}
	return end, nil
}
