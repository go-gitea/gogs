package json

import (
	"bytes"
	"io"
	"unsafe"
)

const (
	initBufSize = 512
)

type stream struct {
	buf                   []byte
	bufSize               int64
	length                int64
	r                     io.Reader
	offset                int64
	cursor                int64
	filledBuffer          bool
	allRead               bool
	useNumber             bool
	disallowUnknownFields bool
}

func newStream(r io.Reader) *stream {
	return &stream{
		r:       r,
		bufSize: initBufSize,
		buf:     []byte{nul},
	}
}

func (s *stream) buffered() io.Reader {
	buflen := int64(len(s.buf))
	for i := s.cursor; i < buflen; i++ {
		if s.buf[i] == nul {
			return bytes.NewReader(s.buf[s.cursor:i])
		}
	}
	return bytes.NewReader(s.buf[s.cursor:])
}

func (s *stream) totalOffset() int64 {
	return s.offset + s.cursor
}

func (s *stream) char() byte {
	return s.buf[s.cursor]
}

func (s *stream) equalChar(c byte) bool {
	cur := s.buf[s.cursor]
	if cur == nul {
		s.read()
		cur = s.buf[s.cursor]
	}
	return cur == c
}

func (s *stream) stat() ([]byte, int64, unsafe.Pointer) {
	return s.buf, s.cursor, (*sliceHeader)(unsafe.Pointer(&s.buf)).data
}

func (s *stream) statForRetry() ([]byte, int64, unsafe.Pointer) {
	s.cursor-- // for retry ( because caller progress cursor position in each loop )
	return s.buf, s.cursor, (*sliceHeader)(unsafe.Pointer(&s.buf)).data
}

func (s *stream) reset() {
	s.offset += s.cursor
	s.buf = s.buf[s.cursor:]
	s.length -= s.cursor
	s.cursor = 0
}

func (s *stream) readBuf() []byte {
	if s.filledBuffer {
		s.bufSize *= 2
		remainBuf := s.buf
		s.buf = make([]byte, s.bufSize)
		copy(s.buf, remainBuf)
	}
	remainLen := s.length - s.cursor
	remainNotNulCharNum := int64(0)
	for i := int64(0); i < remainLen; i++ {
		if s.buf[s.cursor+i] == nul {
			break
		}
		remainNotNulCharNum++
	}
	return s.buf[s.cursor+remainNotNulCharNum:]
}

func (s *stream) read() bool {
	if s.allRead {
		return false
	}
	buf := s.readBuf()
	last := len(buf) - 1
	buf[last] = nul
	n, err := s.r.Read(buf[:last])
	s.length = s.cursor + int64(n)
	if n == last {
		s.filledBuffer = true
	} else {
		s.filledBuffer = false
	}
	if err == io.EOF {
		s.allRead = true
	} else if err != nil {
		return false
	}
	return true
}

func (s *stream) skipWhiteSpace() {
LOOP:
	switch s.char() {
	case ' ', '\n', '\t', '\r':
		s.cursor++
		goto LOOP
	case nul:
		if s.read() {
			goto LOOP
		}
	}
}

func (s *stream) skipObject(depth int64) error {
	braceCount := 1
	_, cursor, p := s.stat()
	for {
		switch char(p, cursor) {
		case '{':
			braceCount++
			depth++
			if depth > maxDecodeNestingDepth {
				return errExceededMaxDepth(s.char(), s.cursor)
			}
		case '}':
			braceCount--
			depth--
			if braceCount == 0 {
				s.cursor = cursor + 1
				return nil
			}
		case '[':
			depth++
			if depth > maxDecodeNestingDepth {
				return errExceededMaxDepth(s.char(), s.cursor)
			}
		case ']':
			depth--
		case '"':
			for {
				cursor++
				switch char(p, cursor) {
				case '\\':
					cursor++
					if char(p, cursor) == nul {
						s.cursor = cursor
						if s.read() {
							_, cursor, p = s.statForRetry()
							continue
						}
						return errUnexpectedEndOfJSON("string of object", cursor)
					}
				case '"':
					goto SWITCH_OUT
				case nul:
					s.cursor = cursor
					if s.read() {
						_, cursor, p = s.statForRetry()
						continue
					}
					return errUnexpectedEndOfJSON("string of object", cursor)
				}
			}
		case nul:
			s.cursor = cursor
			if s.read() {
				_, cursor, p = s.stat()
				continue
			}
			return errUnexpectedEndOfJSON("object of object", cursor)
		}
	SWITCH_OUT:
		cursor++
	}
}

func (s *stream) skipArray(depth int64) error {
	bracketCount := 1
	_, cursor, p := s.stat()
	for {
		switch char(p, cursor) {
		case '[':
			bracketCount++
			depth++
			if depth > maxDecodeNestingDepth {
				return errExceededMaxDepth(s.char(), s.cursor)
			}
		case ']':
			bracketCount--
			depth--
			if bracketCount == 0 {
				s.cursor = cursor + 1
				return nil
			}
		case '{':
			depth++
			if depth > maxDecodeNestingDepth {
				return errExceededMaxDepth(s.char(), s.cursor)
			}
		case '}':
			depth--
		case '"':
			for {
				cursor++
				switch char(p, cursor) {
				case '\\':
					cursor++
					if char(p, cursor) == nul {
						s.cursor = cursor
						if s.read() {
							_, cursor, p = s.statForRetry()
							continue
						}
						return errUnexpectedEndOfJSON("string of object", cursor)
					}
				case '"':
					goto SWITCH_OUT
				case nul:
					s.cursor = cursor
					if s.read() {
						_, cursor, p = s.statForRetry()
						continue
					}
					return errUnexpectedEndOfJSON("string of object", cursor)
				}
			}
		case nul:
			s.cursor = cursor
			if s.read() {
				_, cursor, p = s.stat()
				continue
			}
			return errUnexpectedEndOfJSON("array of object", cursor)
		}
	SWITCH_OUT:
		cursor++
	}
}

func (s *stream) skipValue(depth int64) error {
	_, cursor, p := s.stat()
	for {
		switch char(p, cursor) {
		case ' ', '\n', '\t', '\r':
			cursor++
			continue
		case nul:
			s.cursor = cursor
			if s.read() {
				_, cursor, p = s.stat()
				continue
			}
			return errUnexpectedEndOfJSON("value of object", s.totalOffset())
		case '{':
			s.cursor = cursor + 1
			return s.skipObject(depth + 1)
		case '[':
			s.cursor = cursor + 1
			return s.skipArray(depth + 1)
		case '"':
			for {
				cursor++
				switch char(p, cursor) {
				case '\\':
					cursor++
					if char(p, cursor) == nul {
						s.cursor = cursor
						if s.read() {
							_, cursor, p = s.statForRetry()
							continue
						}
						return errUnexpectedEndOfJSON("value of string", s.totalOffset())
					}
				case '"':
					s.cursor = cursor + 1
					return nil
				case nul:
					s.cursor = cursor
					if s.read() {
						_, cursor, p = s.statForRetry()
						continue
					}
					return errUnexpectedEndOfJSON("value of string", s.totalOffset())
				}
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			for {
				cursor++
				c := char(p, cursor)
				if floatTable[c] {
					continue
				} else if c == nul {
					if s.read() {
						s.cursor-- // for retry current character
						_, cursor, p = s.stat()
						continue
					}
				}
				s.cursor = cursor
				return nil
			}
		case 't':
			s.cursor = cursor
			if err := trueBytes(s); err != nil {
				return err
			}
			return nil
		case 'f':
			s.cursor = cursor
			if err := falseBytes(s); err != nil {
				return err
			}
			return nil
		case 'n':
			s.cursor = cursor
			if err := nullBytes(s); err != nil {
				return err
			}
			return nil
		}
		cursor++
	}
}

func nullBytes(s *stream) error {
	// current cursor's character is 'n'
	s.cursor++
	if s.char() != 'u' {
		if err := retryReadNull(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 'l' {
		if err := retryReadNull(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 'l' {
		if err := retryReadNull(s); err != nil {
			return err
		}
	}
	s.cursor++
	return nil
}

func retryReadNull(s *stream) error {
	if s.char() == nul && s.read() {
		return nil
	}
	return errInvalidCharacter(s.char(), "null", s.totalOffset())
}

func trueBytes(s *stream) error {
	// current cursor's character is 't'
	s.cursor++
	if s.char() != 'r' {
		if err := retryReadTrue(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 'u' {
		if err := retryReadTrue(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 'e' {
		if err := retryReadTrue(s); err != nil {
			return err
		}
	}
	s.cursor++
	return nil
}

func retryReadTrue(s *stream) error {
	if s.char() == nul && s.read() {
		return nil
	}
	return errInvalidCharacter(s.char(), "bool(true)", s.totalOffset())
}

func falseBytes(s *stream) error {
	// current cursor's character is 'f'
	s.cursor++
	if s.char() != 'a' {
		if err := retryReadFalse(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 'l' {
		if err := retryReadFalse(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 's' {
		if err := retryReadFalse(s); err != nil {
			return err
		}
	}
	s.cursor++
	if s.char() != 'e' {
		if err := retryReadFalse(s); err != nil {
			return err
		}
	}
	s.cursor++
	return nil
}

func retryReadFalse(s *stream) error {
	if s.char() == nul && s.read() {
		return nil
	}
	return errInvalidCharacter(s.char(), "bool(false)", s.totalOffset())
}
