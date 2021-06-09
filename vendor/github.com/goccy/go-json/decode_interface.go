package json

import (
	"bytes"
	"encoding"
	"reflect"
	"unsafe"
)

type interfaceDecoder struct {
	typ           *rtype
	structName    string
	fieldName     string
	sliceDecoder  *sliceDecoder
	mapDecoder    *mapDecoder
	floatDecoder  *floatDecoder
	numberDecoder *numberDecoder
	stringDecoder *stringDecoder
}

func newEmptyInterfaceDecoder(structName, fieldName string) *interfaceDecoder {
	ifaceDecoder := &interfaceDecoder{
		typ:        emptyInterfaceType,
		structName: structName,
		fieldName:  fieldName,
		floatDecoder: newFloatDecoder(structName, fieldName, func(p unsafe.Pointer, v float64) {
			*(*interface{})(p) = v
		}),
		numberDecoder: newNumberDecoder(structName, fieldName, func(p unsafe.Pointer, v Number) {
			*(*interface{})(p) = v
		}),
		stringDecoder: newStringDecoder(structName, fieldName),
	}
	ifaceDecoder.sliceDecoder = newSliceDecoder(
		ifaceDecoder,
		emptyInterfaceType,
		emptyInterfaceType.Size(),
		structName, fieldName,
	)
	ifaceDecoder.mapDecoder = newMapDecoder(
		interfaceMapType,
		stringType,
		ifaceDecoder.stringDecoder,
		interfaceMapType.Elem(),
		ifaceDecoder,
		structName,
		fieldName,
	)
	return ifaceDecoder
}

func newInterfaceDecoder(typ *rtype, structName, fieldName string) *interfaceDecoder {
	emptyIfaceDecoder := newEmptyInterfaceDecoder(structName, fieldName)
	stringDecoder := newStringDecoder(structName, fieldName)
	return &interfaceDecoder{
		typ:        typ,
		structName: structName,
		fieldName:  fieldName,
		sliceDecoder: newSliceDecoder(
			emptyIfaceDecoder,
			emptyInterfaceType,
			emptyInterfaceType.Size(),
			structName, fieldName,
		),
		mapDecoder: newMapDecoder(
			interfaceMapType,
			stringType,
			stringDecoder,
			interfaceMapType.Elem(),
			emptyIfaceDecoder,
			structName,
			fieldName,
		),
		floatDecoder: newFloatDecoder(structName, fieldName, func(p unsafe.Pointer, v float64) {
			*(*interface{})(p) = v
		}),
		numberDecoder: newNumberDecoder(structName, fieldName, func(p unsafe.Pointer, v Number) {
			*(*interface{})(p) = v
		}),
		stringDecoder: stringDecoder,
	}
}

func (d *interfaceDecoder) numDecoder(s *stream) decoder {
	if s.useNumber {
		return d.numberDecoder
	}
	return d.floatDecoder
}

var (
	emptyInterfaceType = type2rtype(reflect.TypeOf((*interface{})(nil)).Elem())
	interfaceMapType   = type2rtype(
		reflect.TypeOf((*map[string]interface{})(nil)).Elem(),
	)
	stringType = type2rtype(
		reflect.TypeOf(""),
	)
)

func decodeStreamUnmarshaler(s *stream, depth int64, unmarshaler Unmarshaler) error {
	start := s.cursor
	if err := s.skipValue(depth); err != nil {
		return err
	}
	src := s.buf[start:s.cursor]
	dst := make([]byte, len(src))
	copy(dst, src)

	if err := unmarshaler.UnmarshalJSON(dst); err != nil {
		return err
	}
	return nil
}

func decodeUnmarshaler(buf []byte, cursor, depth int64, unmarshaler Unmarshaler) (int64, error) {
	cursor = skipWhiteSpace(buf, cursor)
	start := cursor
	end, err := skipValue(buf, cursor, depth)
	if err != nil {
		return 0, err
	}
	src := buf[start:end]
	dst := make([]byte, len(src))
	copy(dst, src)

	if err := unmarshaler.UnmarshalJSON(dst); err != nil {
		return 0, err
	}
	return end, nil
}

func decodeStreamTextUnmarshaler(s *stream, depth int64, unmarshaler encoding.TextUnmarshaler, p unsafe.Pointer) error {
	start := s.cursor
	if err := s.skipValue(depth); err != nil {
		return err
	}
	src := s.buf[start:s.cursor]
	if bytes.Equal(src, nullbytes) {
		*(*unsafe.Pointer)(p) = nil
		return nil
	}

	dst := make([]byte, len(src))
	copy(dst, src)

	if err := unmarshaler.UnmarshalText(dst); err != nil {
		return err
	}
	return nil
}

func decodeTextUnmarshaler(buf []byte, cursor, depth int64, unmarshaler encoding.TextUnmarshaler, p unsafe.Pointer) (int64, error) {
	cursor = skipWhiteSpace(buf, cursor)
	start := cursor
	end, err := skipValue(buf, cursor, depth)
	if err != nil {
		return 0, err
	}
	src := buf[start:end]
	if bytes.Equal(src, nullbytes) {
		*(*unsafe.Pointer)(p) = nil
		return end, nil
	}
	if s, ok := unquoteBytes(src); ok {
		src = s
	}
	if err := unmarshaler.UnmarshalText(src); err != nil {
		return 0, err
	}
	return end, nil
}

func (d *interfaceDecoder) decodeStreamEmptyInterface(s *stream, depth int64, p unsafe.Pointer) error {
	s.skipWhiteSpace()
	for {
		switch s.char() {
		case '{':
			var v map[string]interface{}
			ptr := unsafe.Pointer(&v)
			if err := d.mapDecoder.decodeStream(s, depth, ptr); err != nil {
				return err
			}
			*(*interface{})(p) = v
			return nil
		case '[':
			var v []interface{}
			ptr := unsafe.Pointer(&v)
			if err := d.sliceDecoder.decodeStream(s, depth, ptr); err != nil {
				return err
			}
			*(*interface{})(p) = v
			return nil
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return d.numDecoder(s).decodeStream(s, depth, p)
		case '"':
			s.cursor++
			start := s.cursor
			for {
				switch s.char() {
				case '\\':
					if err := decodeEscapeString(s); err != nil {
						return err
					}
				case '"':
					literal := s.buf[start:s.cursor]
					s.cursor++
					*(*interface{})(p) = string(literal)
					return nil
				case nul:
					if s.read() {
						continue
					}
					return errUnexpectedEndOfJSON("string", s.totalOffset())
				}
				s.cursor++
			}
		case 't':
			if err := trueBytes(s); err != nil {
				return err
			}
			**(**interface{})(unsafe.Pointer(&p)) = true
			return nil
		case 'f':
			if err := falseBytes(s); err != nil {
				return err
			}
			**(**interface{})(unsafe.Pointer(&p)) = false
			return nil
		case 'n':
			if err := nullBytes(s); err != nil {
				return err
			}
			*(*interface{})(p) = nil
			return nil
		case nul:
			if s.read() {
				continue
			}
		}
		break
	}
	return errNotAtBeginningOfValue(s.totalOffset())
}

func (d *interfaceDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	runtimeInterfaceValue := *(*interface{})(unsafe.Pointer(&emptyInterface{
		typ: d.typ,
		ptr: p,
	}))
	rv := reflect.ValueOf(runtimeInterfaceValue)
	if rv.NumMethod() > 0 && rv.CanInterface() {
		if u, ok := rv.Interface().(Unmarshaler); ok {
			return decodeStreamUnmarshaler(s, depth, u)
		}
		if u, ok := rv.Interface().(encoding.TextUnmarshaler); ok {
			return decodeStreamTextUnmarshaler(s, depth, u, p)
		}
		s.skipWhiteSpace()
		if s.char() == 'n' {
			if err := nullBytes(s); err != nil {
				return err
			}
			*(*interface{})(p) = nil
			return nil
		}
		return d.errUnmarshalType(rv.Type(), s.totalOffset())
	}
	iface := rv.Interface()
	ifaceHeader := (*emptyInterface)(unsafe.Pointer(&iface))
	typ := ifaceHeader.typ
	if ifaceHeader.ptr == nil || d.typ == typ || typ == nil {
		// concrete type is empty interface
		return d.decodeStreamEmptyInterface(s, depth, p)
	}
	if typ.Kind() == reflect.Ptr && typ.Elem() == d.typ || typ.Kind() != reflect.Ptr {
		return d.decodeStreamEmptyInterface(s, depth, p)
	}
	s.skipWhiteSpace()
	if s.char() == 'n' {
		if err := nullBytes(s); err != nil {
			return err
		}
		*(*interface{})(p) = nil
		return nil
	}
	decoder, err := decodeCompileToGetDecoder(typ)
	if err != nil {
		return err
	}
	return decoder.decodeStream(s, depth, ifaceHeader.ptr)
}

func (d *interfaceDecoder) errUnmarshalType(typ reflect.Type, offset int64) *UnmarshalTypeError {
	return &UnmarshalTypeError{
		Value:  typ.String(),
		Type:   typ,
		Offset: offset,
		Struct: d.structName,
		Field:  d.fieldName,
	}
}

func (d *interfaceDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	runtimeInterfaceValue := *(*interface{})(unsafe.Pointer(&emptyInterface{
		typ: d.typ,
		ptr: p,
	}))
	rv := reflect.ValueOf(runtimeInterfaceValue)
	if rv.NumMethod() > 0 && rv.CanInterface() {
		if u, ok := rv.Interface().(Unmarshaler); ok {
			return decodeUnmarshaler(buf, cursor, depth, u)
		}
		if u, ok := rv.Interface().(encoding.TextUnmarshaler); ok {
			return decodeTextUnmarshaler(buf, cursor, depth, u, p)
		}
		cursor = skipWhiteSpace(buf, cursor)
		if buf[cursor] == 'n' {
			if err := validateNull(buf, cursor); err != nil {
				return 0, err
			}
			cursor += 4
			**(**interface{})(unsafe.Pointer(&p)) = nil
			return cursor, nil
		}
		return 0, d.errUnmarshalType(rv.Type(), cursor)
	}

	iface := rv.Interface()
	ifaceHeader := (*emptyInterface)(unsafe.Pointer(&iface))
	typ := ifaceHeader.typ
	if ifaceHeader.ptr == nil || d.typ == typ || typ == nil {
		// concrete type is empty interface
		return d.decodeEmptyInterface(buf, cursor, depth, p)
	}
	if typ.Kind() == reflect.Ptr && typ.Elem() == d.typ || typ.Kind() != reflect.Ptr {
		return d.decodeEmptyInterface(buf, cursor, depth, p)
	}
	cursor = skipWhiteSpace(buf, cursor)
	if buf[cursor] == 'n' {
		if err := validateNull(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		**(**interface{})(unsafe.Pointer(&p)) = nil
		return cursor, nil
	}
	decoder, err := decodeCompileToGetDecoder(typ)
	if err != nil {
		return 0, err
	}
	return decoder.decode(buf, cursor, depth, ifaceHeader.ptr)
}

func (d *interfaceDecoder) decodeEmptyInterface(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	cursor = skipWhiteSpace(buf, cursor)
	switch buf[cursor] {
	case '{':
		var v map[string]interface{}
		ptr := unsafe.Pointer(&v)
		cursor, err := d.mapDecoder.decode(buf, cursor, depth, ptr)
		if err != nil {
			return 0, err
		}
		**(**interface{})(unsafe.Pointer(&p)) = v
		return cursor, nil
	case '[':
		var v []interface{}
		ptr := unsafe.Pointer(&v)
		cursor, err := d.sliceDecoder.decode(buf, cursor, depth, ptr)
		if err != nil {
			return 0, err
		}
		**(**interface{})(unsafe.Pointer(&p)) = v
		return cursor, nil
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return d.floatDecoder.decode(buf, cursor, depth, p)
	case '"':
		var v string
		ptr := unsafe.Pointer(&v)
		cursor, err := d.stringDecoder.decode(buf, cursor, depth, ptr)
		if err != nil {
			return 0, err
		}
		**(**interface{})(unsafe.Pointer(&p)) = v
		return cursor, nil
	case 't':
		if err := validateTrue(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		**(**interface{})(unsafe.Pointer(&p)) = true
		return cursor, nil
	case 'f':
		if err := validateFalse(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 5
		**(**interface{})(unsafe.Pointer(&p)) = false
		return cursor, nil
	case 'n':
		if err := validateNull(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		**(**interface{})(unsafe.Pointer(&p)) = nil
		return cursor, nil
	}
	return cursor, errNotAtBeginningOfValue(cursor)
}
