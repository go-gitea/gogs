package json

import (
	"unsafe"
)

var (
	isWhiteSpace = [256]bool{}
)

func init() {
	isWhiteSpace[' '] = true
	isWhiteSpace['\n'] = true
	isWhiteSpace['\t'] = true
	isWhiteSpace['\r'] = true
}

func char(ptr unsafe.Pointer, offset int64) byte {
	return *(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(offset)))
}

func skipWhiteSpace(buf []byte, cursor int64) int64 {
	for isWhiteSpace[buf[cursor]] {
		cursor++
	}
	return cursor
}

func skipObject(buf []byte, cursor, depth int64) (int64, error) {
	braceCount := 1
	for {
		switch buf[cursor] {
		case '{':
			braceCount++
			depth++
			if depth > maxDecodeNestingDepth {
				return 0, errExceededMaxDepth(buf[cursor], cursor)
			}
		case '}':
			depth--
			braceCount--
			if braceCount == 0 {
				return cursor + 1, nil
			}
		case '[':
			depth++
			if depth > maxDecodeNestingDepth {
				return 0, errExceededMaxDepth(buf[cursor], cursor)
			}
		case ']':
			depth--
		case '"':
			for {
				cursor++
				switch buf[cursor] {
				case '\\':
					cursor++
					if buf[cursor] == nul {
						return 0, errUnexpectedEndOfJSON("string of object", cursor)
					}
				case '"':
					goto SWITCH_OUT
				case nul:
					return 0, errUnexpectedEndOfJSON("string of object", cursor)
				}
			}
		case nul:
			return 0, errUnexpectedEndOfJSON("object of object", cursor)
		}
	SWITCH_OUT:
		cursor++
	}
}

func skipArray(buf []byte, cursor, depth int64) (int64, error) {
	bracketCount := 1
	for {
		switch buf[cursor] {
		case '[':
			bracketCount++
			depth++
			if depth > maxDecodeNestingDepth {
				return 0, errExceededMaxDepth(buf[cursor], cursor)
			}
		case ']':
			bracketCount--
			depth--
			if bracketCount == 0 {
				return cursor + 1, nil
			}
		case '{':
			depth++
			if depth > maxDecodeNestingDepth {
				return 0, errExceededMaxDepth(buf[cursor], cursor)
			}
		case '}':
			depth--
		case '"':
			for {
				cursor++
				switch buf[cursor] {
				case '\\':
					cursor++
					if buf[cursor] == nul {
						return 0, errUnexpectedEndOfJSON("string of object", cursor)
					}
				case '"':
					goto SWITCH_OUT
				case nul:
					return 0, errUnexpectedEndOfJSON("string of object", cursor)
				}
			}
		case nul:
			return 0, errUnexpectedEndOfJSON("array of object", cursor)
		}
	SWITCH_OUT:
		cursor++
	}
}

func skipValue(buf []byte, cursor, depth int64) (int64, error) {
	for {
		switch buf[cursor] {
		case ' ', '\t', '\n', '\r':
			cursor++
			continue
		case '{':
			return skipObject(buf, cursor+1, depth+1)
		case '[':
			return skipArray(buf, cursor+1, depth+1)
		case '"':
			for {
				cursor++
				switch buf[cursor] {
				case '\\':
					cursor++
					if buf[cursor] == nul {
						return 0, errUnexpectedEndOfJSON("string of object", cursor)
					}
				case '"':
					return cursor + 1, nil
				case nul:
					return 0, errUnexpectedEndOfJSON("string of object", cursor)
				}
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			for {
				cursor++
				if floatTable[buf[cursor]] {
					continue
				}
				break
			}
			return cursor, nil
		case 't':
			if err := validateTrue(buf, cursor); err != nil {
				return 0, err
			}
			cursor += 4
			return cursor, nil
		case 'f':
			if err := validateFalse(buf, cursor); err != nil {
				return 0, err
			}
			cursor += 5
			return cursor, nil
		case 'n':
			if err := validateNull(buf, cursor); err != nil {
				return 0, err
			}
			cursor += 4
			return cursor, nil
		default:
			return cursor, errUnexpectedEndOfJSON("null", cursor)
		}
	}
}

func validateTrue(buf []byte, cursor int64) error {
	if cursor+3 >= int64(len(buf)) {
		return errUnexpectedEndOfJSON("true", cursor)
	}
	if buf[cursor+1] != 'r' {
		return errInvalidCharacter(buf[cursor+1], "true", cursor)
	}
	if buf[cursor+2] != 'u' {
		return errInvalidCharacter(buf[cursor+2], "true", cursor)
	}
	if buf[cursor+3] != 'e' {
		return errInvalidCharacter(buf[cursor+3], "true", cursor)
	}
	return nil
}

func validateFalse(buf []byte, cursor int64) error {
	if cursor+4 >= int64(len(buf)) {
		return errUnexpectedEndOfJSON("false", cursor)
	}
	if buf[cursor+1] != 'a' {
		return errInvalidCharacter(buf[cursor+1], "false", cursor)
	}
	if buf[cursor+2] != 'l' {
		return errInvalidCharacter(buf[cursor+2], "false", cursor)
	}
	if buf[cursor+3] != 's' {
		return errInvalidCharacter(buf[cursor+3], "false", cursor)
	}
	if buf[cursor+4] != 'e' {
		return errInvalidCharacter(buf[cursor+4], "false", cursor)
	}
	return nil
}

func validateNull(buf []byte, cursor int64) error {
	if cursor+3 >= int64(len(buf)) {
		return errUnexpectedEndOfJSON("null", cursor)
	}
	if buf[cursor+1] != 'u' {
		return errInvalidCharacter(buf[cursor+1], "null", cursor)
	}
	if buf[cursor+2] != 'l' {
		return errInvalidCharacter(buf[cursor+2], "null", cursor)
	}
	if buf[cursor+3] != 'l' {
		return errInvalidCharacter(buf[cursor+3], "null", cursor)
	}
	return nil
}
