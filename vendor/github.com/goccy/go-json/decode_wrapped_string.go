package json

import (
	"reflect"
	"unsafe"
)

type wrappedStringDecoder struct {
	typ           *rtype
	dec           decoder
	stringDecoder *stringDecoder
	structName    string
	fieldName     string
	isPtrType     bool
}

func newWrappedStringDecoder(typ *rtype, dec decoder, structName, fieldName string) *wrappedStringDecoder {
	return &wrappedStringDecoder{
		typ:           typ,
		dec:           dec,
		stringDecoder: newStringDecoder(structName, fieldName),
		structName:    structName,
		fieldName:     fieldName,
		isPtrType:     typ.Kind() == reflect.Ptr,
	}
}

func (d *wrappedStringDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	bytes, err := d.stringDecoder.decodeStreamByte(s)
	if err != nil {
		return err
	}
	if bytes == nil {
		if d.isPtrType {
			*(*unsafe.Pointer)(p) = nil
		}
		return nil
	}
	b := make([]byte, len(bytes)+1)
	copy(b, bytes)
	if _, err := d.dec.decode(b, 0, depth, p); err != nil {
		return err
	}
	return nil
}

func (d *wrappedStringDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	bytes, c, err := d.stringDecoder.decodeByte(buf, cursor)
	if err != nil {
		return 0, err
	}
	if bytes == nil {
		if d.isPtrType {
			*(*unsafe.Pointer)(p) = nil
		}
		return c, nil
	}
	bytes = append(bytes, nul)
	if _, err := d.dec.decode(bytes, 0, depth, p); err != nil {
		return 0, err
	}
	return c, nil
}
