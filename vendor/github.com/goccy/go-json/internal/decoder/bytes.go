package decoder

import (
	"encoding/base64"
	"unsafe"

	"github.com/goccy/go-json/internal/errors"
	"github.com/goccy/go-json/internal/runtime"
)

type bytesDecoder struct {
	typ          *runtime.Type
	sliceDecoder Decoder
	structName   string
	fieldName    string
}

func byteUnmarshalerSliceDecoder(typ *runtime.Type, structName string, fieldName string) Decoder {
	var unmarshalDecoder Decoder
	switch {
	case runtime.PtrTo(typ).Implements(unmarshalJSONType):
		unmarshalDecoder = newUnmarshalJSONDecoder(runtime.PtrTo(typ), structName, fieldName)
	case runtime.PtrTo(typ).Implements(unmarshalTextType):
		unmarshalDecoder = newUnmarshalTextDecoder(runtime.PtrTo(typ), structName, fieldName)
	}
	if unmarshalDecoder == nil {
		return nil
	}
	return newSliceDecoder(unmarshalDecoder, typ, 1, structName, fieldName)
}

func newBytesDecoder(typ *runtime.Type, structName string, fieldName string) *bytesDecoder {
	return &bytesDecoder{
		typ:          typ,
		sliceDecoder: byteUnmarshalerSliceDecoder(typ, structName, fieldName),
		structName:   structName,
		fieldName:    fieldName,
	}
}

func (d *bytesDecoder) DecodeStream(s *Stream, depth int64, p unsafe.Pointer) error {
	bytes, err := d.decodeStreamBinary(s, depth, p)
	if err != nil {
		return err
	}
	if bytes == nil {
		s.reset()
		return nil
	}
	decodedLen := base64.StdEncoding.DecodedLen(len(bytes))
	buf := make([]byte, decodedLen)
	n, err := base64.StdEncoding.Decode(buf, bytes)
	if err != nil {
		return err
	}
	*(*[]byte)(p) = buf[:n]
	s.reset()
	return nil
}

func (d *bytesDecoder) Decode(ctx *RuntimeContext, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	bytes, c, err := d.decodeBinary(ctx, cursor, depth, p)
	if err != nil {
		return 0, err
	}
	if bytes == nil {
		return c, nil
	}
	cursor = c
	decodedLen := base64.StdEncoding.DecodedLen(len(bytes))
	b := make([]byte, decodedLen)
	n, err := base64.StdEncoding.Decode(b, bytes)
	if err != nil {
		return 0, err
	}
	*(*[]byte)(p) = b[:n]
	return cursor, nil
}

func binaryBytes(s *Stream) ([]byte, error) {
	s.cursor++
	start := s.cursor
	for {
		switch s.char() {
		case '"':
			literal := s.buf[start:s.cursor]
			s.cursor++
			return literal, nil
		case nul:
			if s.read() {
				continue
			}
			goto ERROR
		}
		s.cursor++
	}
ERROR:
	return nil, errors.ErrUnexpectedEndOfJSON("[]byte", s.totalOffset())
}

func (d *bytesDecoder) decodeStreamBinary(s *Stream, depth int64, p unsafe.Pointer) ([]byte, error) {
	for {
		switch s.char() {
		case ' ', '\n', '\t', '\r':
			s.cursor++
			continue
		case '"':
			return binaryBytes(s)
		case 'n':
			if err := nullBytes(s); err != nil {
				return nil, err
			}
			return nil, nil
		case '[':
			if d.sliceDecoder == nil {
				return nil, &errors.UnmarshalTypeError{
					Type:   runtime.RType2Type(d.typ),
					Offset: s.totalOffset(),
				}
			}
			if err := d.sliceDecoder.DecodeStream(s, depth, p); err != nil {
				return nil, err
			}
			return nil, nil
		case nul:
			if s.read() {
				continue
			}
		}
		break
	}
	return nil, errors.ErrNotAtBeginningOfValue(s.totalOffset())
}

func (d *bytesDecoder) decodeBinary(ctx *RuntimeContext, cursor, depth int64, p unsafe.Pointer) ([]byte, int64, error) {
	buf := ctx.Buf
	for {
		switch buf[cursor] {
		case ' ', '\n', '\t', '\r':
			cursor++
		case '"':
			cursor++
			start := cursor
			for {
				switch buf[cursor] {
				case '"':
					literal := buf[start:cursor]
					cursor++
					return literal, cursor, nil
				case nul:
					return nil, 0, errors.ErrUnexpectedEndOfJSON("[]byte", cursor)
				}
				cursor++
			}
		case '[':
			if d.sliceDecoder == nil {
				return nil, 0, &errors.UnmarshalTypeError{
					Type:   runtime.RType2Type(d.typ),
					Offset: cursor,
				}
			}
			c, err := d.sliceDecoder.Decode(ctx, cursor, depth, p)
			if err != nil {
				return nil, 0, err
			}
			return nil, c, nil
		case 'n':
			if err := validateNull(buf, cursor); err != nil {
				return nil, 0, err
			}
			cursor += 4
			return nil, cursor, nil
		default:
			return nil, 0, errors.ErrNotAtBeginningOfValue(cursor)
		}
	}
}
