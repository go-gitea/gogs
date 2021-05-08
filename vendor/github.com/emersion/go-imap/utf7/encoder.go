package utf7

import (
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/transform"
)

type encoder struct{}

func (e *encoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for i := 0; i < len(src); {
		ch := src[i]

		var b []byte
		if min <= ch && ch <= max {
			b = []byte{ch}
			if ch == '&' {
				b = append(b, '-')
			}

			i++
		} else {
			start := i

			// Find the next printable ASCII code point
			i++
			for i < len(src) && (src[i] < min || src[i] > max) {
				i++
			}

			if !atEOF && i == len(src) {
				err = transform.ErrShortSrc
				return
			}

			b = encode(src[start:i])
		}

		if nDst+len(b) > len(dst) {
			err = transform.ErrShortDst
			return
		}

		nSrc = i

		for _, ch := range b {
			dst[nDst] = ch
			nDst++
		}
	}

	return
}

func (e *encoder) Reset() {}

// Converts string s from UTF-8 to UTF-16-BE, encodes the result as base64,
// removes the padding, and adds UTF-7 shifts.
func encode(s []byte) []byte {
	// len(s) is sufficient for UTF-8 to UTF-16 conversion if there are no
	// control code points (see table below).
	b := make([]byte, 0, len(s)+4)
	for len(s) > 0 {
		r, size := utf8.DecodeRune(s)
		if r > utf8.MaxRune {
			r, size = utf8.RuneError, 1 // Bug fix (issue 3785)
		}
		s = s[size:]
		if r1, r2 := utf16.EncodeRune(r); r1 != repl {
			b = append(b, byte(r1>>8), byte(r1))
			r = r2
		}
		b = append(b, byte(r>>8), byte(r))
	}

	// Encode as base64
	n := b64Enc.EncodedLen(len(b)) + 2
	b64 := make([]byte, n)
	b64Enc.Encode(b64[1:], b)

	// Strip padding
	n -= 2 - (len(b)+2)%3
	b64 = b64[:n]

	// Add UTF-7 shifts
	b64[0] = '&'
	b64[n-1] = '-'
	return b64
}
