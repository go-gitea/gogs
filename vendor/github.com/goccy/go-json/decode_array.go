package json

import (
	"unsafe"
)

type arrayDecoder struct {
	elemType     *rtype
	size         uintptr
	valueDecoder decoder
	alen         int
	structName   string
	fieldName    string
	zeroValue    unsafe.Pointer
}

func newArrayDecoder(dec decoder, elemType *rtype, alen int, structName, fieldName string) *arrayDecoder {
	zeroValue := *(*unsafe.Pointer)(unsafe_New(elemType))
	return &arrayDecoder{
		valueDecoder: dec,
		elemType:     elemType,
		size:         elemType.Size(),
		alen:         alen,
		structName:   structName,
		fieldName:    fieldName,
		zeroValue:    zeroValue,
	}
}

func (d *arrayDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
	depth++
	if depth > maxDecodeNestingDepth {
		return errExceededMaxDepth(s.char(), s.cursor)
	}

	for {
		switch s.char() {
		case ' ', '\n', '\t', '\r':
		case 'n':
			if err := nullBytes(s); err != nil {
				return err
			}
			return nil
		case '[':
			idx := 0
			for {
				s.cursor++
				if idx < d.alen {
					if err := d.valueDecoder.decodeStream(s, depth, unsafe.Pointer(uintptr(p)+uintptr(idx)*d.size)); err != nil {
						return err
					}
				} else {
					if err := s.skipValue(depth); err != nil {
						return err
					}
				}
				idx++
				s.skipWhiteSpace()
				switch s.char() {
				case ']':
					for idx < d.alen {
						*(*unsafe.Pointer)(unsafe.Pointer(uintptr(p) + uintptr(idx)*d.size)) = d.zeroValue
						idx++
					}
					s.cursor++
					return nil
				case ',':
					continue
				case nul:
					if s.read() {
						continue
					}
					goto ERROR
				default:
					goto ERROR
				}
			}
		case nul:
			if s.read() {
				continue
			}
			goto ERROR
		default:
			goto ERROR
		}
		s.cursor++
	}
ERROR:
	return errUnexpectedEndOfJSON("array", s.totalOffset())
}

func (d *arrayDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	depth++
	if depth > maxDecodeNestingDepth {
		return 0, errExceededMaxDepth(buf[cursor], cursor)
	}

	for {
		switch buf[cursor] {
		case ' ', '\n', '\t', '\r':
			cursor++
			continue
		case 'n':
			if err := validateNull(buf, cursor); err != nil {
				return 0, err
			}
			cursor += 4
			return cursor, nil
		case '[':
			idx := 0
			for {
				cursor++
				if idx < d.alen {
					c, err := d.valueDecoder.decode(buf, cursor, depth, unsafe.Pointer(uintptr(p)+uintptr(idx)*d.size))
					if err != nil {
						return 0, err
					}
					cursor = c
				} else {
					c, err := skipValue(buf, cursor, depth)
					if err != nil {
						return 0, err
					}
					cursor = c
				}
				idx++
				cursor = skipWhiteSpace(buf, cursor)
				switch buf[cursor] {
				case ']':
					for idx < d.alen {
						*(*unsafe.Pointer)(unsafe.Pointer(uintptr(p) + uintptr(idx)*d.size)) = d.zeroValue
						idx++
					}
					cursor++
					return cursor, nil
				case ',':
					continue
				default:
					return 0, errInvalidCharacter(buf[cursor], "array", cursor)
				}
			}
		default:
			return 0, errUnexpectedEndOfJSON("array", cursor)
		}
	}
}
