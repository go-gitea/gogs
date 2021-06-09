package json

import (
	"unsafe"
)

type boolDecoder struct {
	structName string
	fieldName  string
}

func newBoolDecoder(structName, fieldName string) *boolDecoder {
	return &boolDecoder{structName: structName, fieldName: fieldName}
}

func (d *boolDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	s.skipWhiteSpace()
	for {
		switch s.char() {
		case 't':
			if err := trueBytes(s); err != nil {
				return err
			}
			**(**bool)(unsafe.Pointer(&p)) = true
			return nil
		case 'f':
			if err := falseBytes(s); err != nil {
				return err
			}
			**(**bool)(unsafe.Pointer(&p)) = false
			return nil
		case 'n':
			if err := nullBytes(s); err != nil {
				return err
			}
			return nil
		case nul:
			if s.read() {
				continue
			}
			goto ERROR
		}
		break
	}
ERROR:
	return errUnexpectedEndOfJSON("bool", s.totalOffset())
}

func (d *boolDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	cursor = skipWhiteSpace(buf, cursor)
	switch buf[cursor] {
	case 't':
		if err := validateTrue(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		**(**bool)(unsafe.Pointer(&p)) = true
		return cursor, nil
	case 'f':
		if err := validateFalse(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 5
		**(**bool)(unsafe.Pointer(&p)) = false
		return cursor, nil
	case 'n':
		if err := validateNull(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		return cursor, nil
	}
	return 0, errUnexpectedEndOfJSON("bool", cursor)
}
