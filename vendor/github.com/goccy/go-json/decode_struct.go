package json

import (
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"
	"unsafe"
)

type structFieldSet struct {
	dec         decoder
	offset      uintptr
	isTaggedKey bool
	key         string
	keyLen      int64
	err         error
}

type structDecoder struct {
	fieldMap         map[string]*structFieldSet
	stringDecoder    *stringDecoder
	structName       string
	fieldName        string
	isTriedOptimize  bool
	keyBitmapUint8   [][256]uint8
	keyBitmapUint16  [][256]uint16
	sortedFieldSets  []*structFieldSet
	keyDecoder       func(*structDecoder, []byte, int64) (int64, *structFieldSet, error)
	keyStreamDecoder func(*structDecoder, *stream) (*structFieldSet, string, error)
}

var (
	largeToSmallTable [256]byte
)

func init() {
	for i := 0; i < 256; i++ {
		c := i
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		largeToSmallTable[i] = byte(c)
	}
}

func newStructDecoder(structName, fieldName string, fieldMap map[string]*structFieldSet) *structDecoder {
	return &structDecoder{
		fieldMap:         fieldMap,
		stringDecoder:    newStringDecoder(structName, fieldName),
		structName:       structName,
		fieldName:        fieldName,
		keyDecoder:       decodeKey,
		keyStreamDecoder: decodeKeyStream,
	}
}

const (
	allowOptimizeMaxKeyLen   = 64
	allowOptimizeMaxFieldLen = 16
)

func (d *structDecoder) tryOptimize() {
	if d.isTriedOptimize {
		return
	}
	fieldMap := map[string]*structFieldSet{}
	conflicted := map[string]struct{}{}
	for k, v := range d.fieldMap {
		key := strings.ToLower(k)
		if key != k {
			// already exists same key (e.g. Hello and HELLO has same lower case key
			if _, exists := conflicted[key]; exists {
				d.isTriedOptimize = true
				return
			}
			conflicted[key] = struct{}{}
		}
		if field, exists := fieldMap[key]; exists {
			if field != v {
				d.isTriedOptimize = true
				return
			}
		}
		fieldMap[key] = v
	}

	if len(fieldMap) > allowOptimizeMaxFieldLen {
		d.isTriedOptimize = true
		return
	}

	var maxKeyLen int
	sortedKeys := []string{}
	for key := range fieldMap {
		keyLen := len(key)
		if keyLen > allowOptimizeMaxKeyLen {
			d.isTriedOptimize = true
			return
		}
		if maxKeyLen < keyLen {
			maxKeyLen = keyLen
		}
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	// By allocating one extra capacity than `maxKeyLen`,
	// it is possible to avoid the process of comparing the index of the key with the length of the bitmap each time.
	bitmapLen := maxKeyLen + 1
	if len(sortedKeys) <= 8 {
		keyBitmap := make([][256]uint8, bitmapLen)
		for i, key := range sortedKeys {
			for j := 0; j < len(key); j++ {
				c := key[j]
				keyBitmap[j][c] |= (1 << uint(i))
			}
			d.sortedFieldSets = append(d.sortedFieldSets, fieldMap[key])
		}
		d.keyBitmapUint8 = keyBitmap
		d.keyDecoder = decodeKeyByBitmapUint8
		d.keyStreamDecoder = decodeKeyByBitmapUint8Stream
	} else {
		keyBitmap := make([][256]uint16, bitmapLen)
		for i, key := range sortedKeys {
			for j := 0; j < len(key); j++ {
				c := key[j]
				keyBitmap[j][c] |= (1 << uint(i))
			}
			d.sortedFieldSets = append(d.sortedFieldSets, fieldMap[key])
		}
		d.keyBitmapUint16 = keyBitmap
		d.keyDecoder = decodeKeyByBitmapUint16
		d.keyStreamDecoder = decodeKeyByBitmapUint16Stream
	}
}

func decodeKeyByBitmapUint8(d *structDecoder, buf []byte, cursor int64) (int64, *structFieldSet, error) {
	var (
		field  *structFieldSet
		curBit uint8 = math.MaxUint8
	)
	b := (*sliceHeader)(unsafe.Pointer(&buf)).data
	for {
		switch char(b, cursor) {
		case ' ', '\n', '\t', '\r':
			cursor++
		case '"':
			cursor++
			c := char(b, cursor)
			switch c {
			case '"':
				cursor++
				return cursor, field, nil
			case nul:
				return 0, nil, errUnexpectedEndOfJSON("string", cursor)
			}
			keyIdx := 0
			bitmap := d.keyBitmapUint8
			start := cursor
			for {
				c := char(b, cursor)
				switch c {
				case '"':
					fieldSetIndex := bits.TrailingZeros8(curBit)
					field = d.sortedFieldSets[fieldSetIndex]
					keyLen := cursor - start
					cursor++
					if keyLen < field.keyLen {
						// early match
						return cursor, nil, nil
					}
					return cursor, field, nil
				case nul:
					return 0, nil, errUnexpectedEndOfJSON("string", cursor)
				default:
					curBit &= bitmap[keyIdx][largeToSmallTable[c]]
					if curBit == 0 {
						for {
							cursor++
							switch char(b, cursor) {
							case '"':
								cursor++
								return cursor, field, nil
							case '\\':
								cursor++
								if char(b, cursor) == nul {
									return 0, nil, errUnexpectedEndOfJSON("string", cursor)
								}
							case nul:
								return 0, nil, errUnexpectedEndOfJSON("string", cursor)
							}
						}
					}
					keyIdx++
				}
				cursor++
			}
		default:
			return cursor, nil, errNotAtBeginningOfValue(cursor)
		}
	}
}

func decodeKeyByBitmapUint16(d *structDecoder, buf []byte, cursor int64) (int64, *structFieldSet, error) {
	var (
		field  *structFieldSet
		curBit uint16 = math.MaxUint16
	)
	b := (*sliceHeader)(unsafe.Pointer(&buf)).data
	for {
		switch char(b, cursor) {
		case ' ', '\n', '\t', '\r':
			cursor++
		case '"':
			cursor++
			c := char(b, cursor)
			switch c {
			case '"':
				cursor++
				return cursor, field, nil
			case nul:
				return 0, nil, errUnexpectedEndOfJSON("string", cursor)
			}
			keyIdx := 0
			bitmap := d.keyBitmapUint16
			start := cursor
			for {
				c := char(b, cursor)
				switch c {
				case '"':
					fieldSetIndex := bits.TrailingZeros16(curBit)
					field = d.sortedFieldSets[fieldSetIndex]
					keyLen := cursor - start
					cursor++
					if keyLen < field.keyLen {
						// early match
						return cursor, nil, nil
					}
					return cursor, field, nil
				case nul:
					return 0, nil, errUnexpectedEndOfJSON("string", cursor)
				default:
					curBit &= bitmap[keyIdx][largeToSmallTable[c]]
					if curBit == 0 {
						for {
							cursor++
							switch char(b, cursor) {
							case '"':
								cursor++
								return cursor, field, nil
							case '\\':
								cursor++
								if char(b, cursor) == nul {
									return 0, nil, errUnexpectedEndOfJSON("string", cursor)
								}
							case nul:
								return 0, nil, errUnexpectedEndOfJSON("string", cursor)
							}
						}
					}
					keyIdx++
				}
				cursor++
			}
		default:
			return cursor, nil, errNotAtBeginningOfValue(cursor)
		}
	}
}

func decodeKey(d *structDecoder, buf []byte, cursor int64) (int64, *structFieldSet, error) {
	key, c, err := d.stringDecoder.decodeByte(buf, cursor)
	if err != nil {
		return 0, nil, err
	}
	cursor = c
	k := *(*string)(unsafe.Pointer(&key))
	field, exists := d.fieldMap[k]
	if !exists {
		return cursor, nil, nil
	}
	return cursor, field, nil
}

func decodeKeyByBitmapUint8Stream(d *structDecoder, s *stream) (*structFieldSet, string, error) {
	var (
		field  *structFieldSet
		curBit uint8 = math.MaxUint8
	)
	buf, cursor, p := s.stat()
	for {
		switch char(p, cursor) {
		case ' ', '\n', '\t', '\r':
			cursor++
		case nul:
			s.cursor = cursor
			if s.read() {
				buf, cursor, p = s.stat()
				continue
			}
			return nil, "", errNotAtBeginningOfValue(s.totalOffset())
		case '"':
			cursor++
		FIRST_CHAR:
			start := cursor
			switch char(p, cursor) {
			case '"':
				cursor++
				s.cursor = cursor
				return field, "", nil
			case nul:
				s.cursor = cursor
				if s.read() {
					buf, cursor, p = s.stat()
					goto FIRST_CHAR
				}
				return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
			}
			keyIdx := 0
			bitmap := d.keyBitmapUint8
			for {
				c := char(p, cursor)
				switch c {
				case '"':
					fieldSetIndex := bits.TrailingZeros8(curBit)
					field = d.sortedFieldSets[fieldSetIndex]
					keyLen := cursor - start
					cursor++
					s.cursor = cursor
					if keyLen < field.keyLen {
						// early match
						return nil, field.key, nil
					}
					return field, field.key, nil
				case nul:
					s.cursor = cursor
					if s.read() {
						buf, cursor, p = s.stat()
						continue
					}
					return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
				default:
					curBit &= bitmap[keyIdx][largeToSmallTable[c]]
					if curBit == 0 {
						for {
							cursor++
							switch char(p, cursor) {
							case '"':
								b := buf[start:cursor]
								key := *(*string)(unsafe.Pointer(&b))
								cursor++
								s.cursor = cursor
								return field, key, nil
							case '\\':
								cursor++
								if char(p, cursor) == nul {
									s.cursor = cursor
									if !s.read() {
										return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
									}
									buf, cursor, p = s.statForRetry()
								}
							case nul:
								s.cursor = cursor
								if !s.read() {
									return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
								}
								buf, cursor, p = s.statForRetry()
							}
						}
					}
					keyIdx++
				}
				cursor++
			}
		default:
			return nil, "", errNotAtBeginningOfValue(s.totalOffset())
		}
	}
}

func decodeKeyByBitmapUint16Stream(d *structDecoder, s *stream) (*structFieldSet, string, error) {
	var (
		field  *structFieldSet
		curBit uint16 = math.MaxUint16
	)
	buf, cursor, p := s.stat()
	for {
		switch char(p, cursor) {
		case ' ', '\n', '\t', '\r':
			cursor++
		case nul:
			s.cursor = cursor
			if s.read() {
				buf, cursor, p = s.stat()
				continue
			}
			return nil, "", errNotAtBeginningOfValue(s.totalOffset())
		case '"':
			cursor++
		FIRST_CHAR:
			start := cursor
			switch char(p, cursor) {
			case '"':
				cursor++
				s.cursor = cursor
				return field, "", nil
			case nul:
				s.cursor = cursor
				if s.read() {
					buf, cursor, p = s.stat()
					goto FIRST_CHAR
				}
				return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
			}
			keyIdx := 0
			bitmap := d.keyBitmapUint16
			for {
				c := char(p, cursor)
				switch c {
				case '"':
					fieldSetIndex := bits.TrailingZeros16(curBit)
					field = d.sortedFieldSets[fieldSetIndex]
					keyLen := cursor - start
					cursor++
					s.cursor = cursor
					if keyLen < field.keyLen {
						// early match
						return nil, field.key, nil
					}
					return field, field.key, nil
				case nul:
					s.cursor = cursor
					if s.read() {
						buf, cursor, p = s.stat()
						continue
					}
					return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
				default:
					curBit &= bitmap[keyIdx][largeToSmallTable[c]]
					if curBit == 0 {
						for {
							cursor++
							switch char(p, cursor) {
							case '"':
								b := buf[start:cursor]
								key := *(*string)(unsafe.Pointer(&b))
								cursor++
								s.cursor = cursor
								return field, key, nil
							case '\\':
								cursor++
								if char(p, cursor) == nul {
									s.cursor = cursor
									if !s.read() {
										return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
									}
									buf, cursor, p = s.statForRetry()
								}
							case nul:
								s.cursor = cursor
								if !s.read() {
									return nil, "", errUnexpectedEndOfJSON("string", s.totalOffset())
								}
								buf, cursor, p = s.statForRetry()
							}
						}
					}
					keyIdx++
				}
				cursor++
			}
		default:
			return nil, "", errNotAtBeginningOfValue(s.totalOffset())
		}
	}
}

func decodeKeyStream(d *structDecoder, s *stream) (*structFieldSet, string, error) {
	key, err := d.stringDecoder.decodeStreamByte(s)
	if err != nil {
		return nil, "", err
	}
	k := *(*string)(unsafe.Pointer(&key))
	return d.fieldMap[k], k, nil
}

func (d *structDecoder) decodeStream(s *stream, depth int64, p unsafe.Pointer) error {
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
		return nil
	case nul:
		s.read()
	default:
		if s.char() != '{' {
			return errNotAtBeginningOfValue(s.totalOffset())
		}
	}
	s.cursor++
	s.skipWhiteSpace()
	if s.char() == '}' {
		s.cursor++
		return nil
	}
	for {
		s.reset()
		field, key, err := d.keyStreamDecoder(d, s)
		if err != nil {
			return err
		}
		s.skipWhiteSpace()
		if s.char() != ':' {
			return errExpected("colon after object key", s.totalOffset())
		}
		s.cursor++
		if s.char() == nul {
			if !s.read() {
				return errExpected("object value after colon", s.totalOffset())
			}
		}
		if field != nil {
			if field.err != nil {
				return field.err
			}
			if err := field.dec.decodeStream(s, depth, unsafe.Pointer(uintptr(p)+field.offset)); err != nil {
				return err
			}
		} else if s.disallowUnknownFields {
			return fmt.Errorf("json: unknown field %q", key)
		} else {
			if err := s.skipValue(depth); err != nil {
				return err
			}
		}
		s.skipWhiteSpace()
		c := s.char()
		if c == '}' {
			s.cursor++
			return nil
		}
		if c != ',' {
			return errExpected("comma after object element", s.totalOffset())
		}
		s.cursor++
	}
}

func (d *structDecoder) decode(buf []byte, cursor, depth int64, p unsafe.Pointer) (int64, error) {
	depth++
	if depth > maxDecodeNestingDepth {
		return 0, errExceededMaxDepth(buf[cursor], cursor)
	}
	buflen := int64(len(buf))
	cursor = skipWhiteSpace(buf, cursor)
	b := (*sliceHeader)(unsafe.Pointer(&buf)).data
	switch char(b, cursor) {
	case 'n':
		if err := validateNull(buf, cursor); err != nil {
			return 0, err
		}
		cursor += 4
		return cursor, nil
	case '{':
	default:
		return 0, errNotAtBeginningOfValue(cursor)
	}
	cursor++
	cursor = skipWhiteSpace(buf, cursor)
	if buf[cursor] == '}' {
		cursor++
		return cursor, nil
	}
	for {
		c, field, err := d.keyDecoder(d, buf, cursor)
		if err != nil {
			return 0, err
		}
		cursor = skipWhiteSpace(buf, c)
		if char(b, cursor) != ':' {
			return 0, errExpected("colon after object key", cursor)
		}
		cursor++
		if cursor >= buflen {
			return 0, errExpected("object value after colon", cursor)
		}
		if field != nil {
			if field.err != nil {
				return 0, field.err
			}
			c, err := field.dec.decode(buf, cursor, depth, unsafe.Pointer(uintptr(p)+field.offset))
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
		cursor = skipWhiteSpace(buf, cursor)
		if char(b, cursor) == '}' {
			cursor++
			return cursor, nil
		}
		if char(b, cursor) != ',' {
			return 0, errExpected("comma after object element", cursor)
		}
		cursor++
	}
}
