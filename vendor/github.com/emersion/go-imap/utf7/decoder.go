package utf7

import (
	"errors"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/transform"
)

// ErrInvalidUTF7 means that a transformer encountered invalid UTF-7.
var ErrInvalidUTF7 = errors.New("utf7: invalid UTF-7")

type decoder struct {
	ascii bool
}

func (d *decoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for i := 0; i < len(src); i++ {
		ch := src[i]

		if ch < min || ch > max { // Illegal code point in ASCII mode
			err = ErrInvalidUTF7
			return
		}

		if ch != '&' {
			if nDst+1 > len(dst) {
				err = transform.ErrShortDst
				return
			}

			nSrc++

			dst[nDst] = ch
			nDst++

			d.ascii = true
			continue
		}

		// Find the end of the Base64 or "&-" segment
		start := i + 1
		for i++; i < len(src) && src[i] != '-'; i++ {
			if src[i] == '\r' || src[i] == '\n' { // base64 package ignores CR and LF
				err = ErrInvalidUTF7
				return
			}
		}

		if i == len(src) { // Implicit shift ("&...")
			if atEOF {
				err = ErrInvalidUTF7
			} else {
				err = transform.ErrShortSrc
			}
			return
		}

		var b []byte
		if i == start { // Escape sequence "&-"
			b = []byte{'&'}
			d.ascii = true
		} else { // Control or non-ASCII code points in base64
			if !d.ascii { // Null shift ("&...-&...-")
				err = ErrInvalidUTF7
				return
			}

			b = decode(src[start:i])
			d.ascii = false
		}

		if len(b) == 0 { // Bad encoding
			err = ErrInvalidUTF7
			return
		}

		if nDst+len(b) > len(dst) {
			if atEOF {
				d.ascii = true
			}
			err = transform.ErrShortDst
			return
		}

		nSrc = i + 1

		for _, ch := range b {
			dst[nDst] = ch
			nDst++
		}
	}

	if atEOF {
		d.ascii = true
	}

	return
}

func (d *decoder) Reset() {
	d.ascii = true
}

// Extracts UTF-16-BE bytes from base64 data and converts them to UTF-8.
// A nil slice is returned if the encoding is invalid.
func decode(b64 []byte) []byte {
	var b []byte

	// Allocate a single block of memory large enough to store the Base64 data
	// (if padding is required), UTF-16-BE bytes, and decoded UTF-8 bytes.
	// Since a 2-byte UTF-16 sequence may expand into a 3-byte UTF-8 sequence,
	// double the space allocation for UTF-8.
	if n := len(b64); b64[n-1] == '=' {
		return nil
	} else if n&3 == 0 {
		b = make([]byte, b64Enc.DecodedLen(n)*3)
	} else {
		n += 4 - n&3
		b = make([]byte, n+b64Enc.DecodedLen(n)*3)
		copy(b[copy(b, b64):n], []byte("=="))
		b64, b = b[:n], b[n:]
	}

	// Decode Base64 into the first 1/3rd of b
	n, err := b64Enc.Decode(b, b64)
	if err != nil || n&1 == 1 {
		return nil
	}

	// Decode UTF-16-BE into the remaining 2/3rds of b
	b, s := b[:n], b[n:]
	j := 0
	for i := 0; i < n; i += 2 {
		r := rune(b[i])<<8 | rune(b[i+1])
		if utf16.IsSurrogate(r) {
			if i += 2; i == n {
				return nil
			}
			r2 := rune(b[i])<<8 | rune(b[i+1])
			if r = utf16.DecodeRune(r, r2); r == repl {
				return nil
			}
		} else if min <= r && r <= max {
			return nil
		}
		j += utf8.EncodeRune(s[j:], r)
	}
	return s[:j]
}
