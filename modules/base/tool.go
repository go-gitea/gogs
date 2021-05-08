// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/dustin/go-humanize"
)

// Use at most this many bytes to determine Content Type.
const sniffLen = 512

// SVGMimeType MIME type of SVG images.
const SVGMimeType = "image/svg+xml"

var svgTagRegex = regexp.MustCompile(`(?si)\A\s*(?:(<!--.*?-->|<!DOCTYPE\s+svg([\s:]+.*?>|>))\s*)*<svg[\s>\/]`)
var svgTagInXMLRegex = regexp.MustCompile(`(?si)\A<\?xml\b.*?\?>\s*(?:(<!--.*?-->|<!DOCTYPE\s+svg([\s:]+.*?>|>))\s*)*<svg[\s>\/]`)

// EncodeMD5 encodes string to md5 hex value.
func EncodeMD5(str string) string {
	m := md5.New()
	_, _ = m.Write([]byte(str))
	return hex.EncodeToString(m.Sum(nil))
}

// EncodeSha1 string to sha1 hex value.
func EncodeSha1(str string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// EncodeSha256 string to sha256 hex value.
func EncodeSha256(str string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// ShortSha is basically just truncating.
// It is DEPRECATED and will be removed in the future.
func ShortSha(sha1 string) string {
	return TruncateString(sha1, 10)
}

// BasicAuthDecode decode basic auth string
func BasicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	auth := strings.SplitN(string(s), ":", 2)

	if len(auth) != 2 {
		return "", "", errors.New("invalid basic authentication")
	}

	return auth[0], auth[1], nil
}

// BasicAuthEncode encode basic auth string
func BasicAuthEncode(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// VerifyTimeLimitCode verify time limit code
func VerifyTimeLimitCode(data string, minutes int, code string) bool {
	if len(code) <= 18 {
		return false
	}

	// split code
	start := code[:12]
	lives := code[12:18]
	if d, err := strconv.ParseInt(lives, 10, 0); err == nil {
		minutes = int(d)
	}

	// right active code
	retCode := CreateTimeLimitCode(data, minutes, start)
	if retCode == code && minutes > 0 {
		// check time is expired or not
		before, _ := time.ParseInLocation("200601021504", start, time.Local)
		now := time.Now()
		if before.Add(time.Minute*time.Duration(minutes)).Unix() > now.Unix() {
			return true
		}
	}

	return false
}

// TimeLimitCodeLength default value for time limit code
const TimeLimitCodeLength = 12 + 6 + 40

// CreateTimeLimitCode create a time limit code
// code format: 12 length date time string + 6 minutes string + 40 sha1 encoded string
func CreateTimeLimitCode(data string, minutes int, startInf interface{}) string {
	format := "200601021504"

	var start, end time.Time
	var startStr, endStr string

	if startInf == nil {
		// Use now time create code
		start = time.Now()
		startStr = start.Format(format)
	} else {
		// use start string create code
		startStr = startInf.(string)
		start, _ = time.ParseInLocation(format, startStr, time.Local)
		startStr = start.Format(format)
	}

	end = start.Add(time.Minute * time.Duration(minutes))
	endStr = end.Format(format)

	// create sha1 encode string
	sh := sha1.New()
	_, _ = sh.Write([]byte(fmt.Sprintf("%s%s%s%s%d", data, setting.SecretKey, startStr, endStr, minutes)))
	encoded := hex.EncodeToString(sh.Sum(nil))

	code := fmt.Sprintf("%s%06d%s", startStr, minutes, encoded)
	return code
}

// FileSize calculates the file size and generate user-friendly string.
func FileSize(s int64) string {
	return humanize.IBytes(uint64(s))
}

// PrettyNumber produces a string form of the given number in base 10 with
// commas after every three orders of magnitud
func PrettyNumber(v int64) string {
	return humanize.Comma(v)
}

// Subtract deals with subtraction of all types of number.
func Subtract(left interface{}, right interface{}) interface{} {
	var rleft, rright int64
	var fleft, fright float64
	var isInt = true
	switch v := left.(type) {
	case int:
		rleft = int64(v)
	case int8:
		rleft = int64(v)
	case int16:
		rleft = int64(v)
	case int32:
		rleft = int64(v)
	case int64:
		rleft = v
	case float32:
		fleft = float64(v)
		isInt = false
	case float64:
		fleft = v
		isInt = false
	}

	switch v := right.(type) {
	case int:
		rright = int64(v)
	case int8:
		rright = int64(v)
	case int16:
		rright = int64(v)
	case int32:
		rright = int64(v)
	case int64:
		rright = v
	case float32:
		fright = float64(v)
		isInt = false
	case float64:
		fright = v
		isInt = false
	}

	if isInt {
		return rleft - rright
	}
	return fleft + float64(rleft) - (fright + float64(rright))
}

// EllipsisString returns a truncated short string,
// it appends '...' in the end of the length of string is too large.
func EllipsisString(str string, length int) string {
	if length <= 3 {
		return "..."
	}
	if len(str) <= length {
		return str
	}
	return str[:length-3] + "..."
}

// TruncateString returns a truncated string with given limit,
// it returns input string if length is not reached limit.
func TruncateString(str string, limit int) string {
	if len(str) < limit {
		return str
	}
	return str[:limit]
}

// StringsToInt64s converts a slice of string to a slice of int64.
func StringsToInt64s(strs []string) ([]int64, error) {
	ints := make([]int64, len(strs))
	for i := range strs {
		n, err := strconv.ParseInt(strs[i], 10, 64)
		if err != nil {
			return ints, err
		}
		ints[i] = n
	}
	return ints, nil
}

// Int64sToStrings converts a slice of int64 to a slice of string.
func Int64sToStrings(ints []int64) []string {
	strs := make([]string, len(ints))
	for i := range ints {
		strs[i] = strconv.FormatInt(ints[i], 10)
	}
	return strs
}

// Int64sToMap converts a slice of int64 to a int64 map.
func Int64sToMap(ints []int64) map[int64]bool {
	m := make(map[int64]bool)
	for _, i := range ints {
		m[i] = true
	}
	return m
}

// Int64sContains returns if a int64 in a slice of int64
func Int64sContains(intsSlice []int64, a int64) bool {
	for _, c := range intsSlice {
		if c == a {
			return true
		}
	}
	return false
}

// IsLetter reports whether the rune is a letter (category L).
// https://github.com/golang/go/blob/c3b4918/src/go/scanner/scanner.go#L342
func IsLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= 0x80 && unicode.IsLetter(ch)
}

// DetectContentType extends http.DetectContentType with more content types.
func DetectContentType(data []byte) string {
	ct := http.DetectContentType(data)

	if len(data) > sniffLen {
		data = data[:sniffLen]
	}

	if setting.UI.SVG.Enabled &&
		((strings.Contains(ct, "text/plain") || strings.Contains(ct, "text/html")) && svgTagRegex.Match(data) ||
			strings.Contains(ct, "text/xml") && svgTagInXMLRegex.Match(data)) {

		// SVG is unsupported.  https://github.com/golang/go/issues/15888
		return SVGMimeType
	}
	return ct
}

// IsRepresentableAsText returns true if file content can be represented as
// plain text or is empty.
func IsRepresentableAsText(data []byte) bool {
	return IsTextFile(data) || IsSVGImageFile(data)
}

// IsTextFile returns true if file content format is plain text or empty.
func IsTextFile(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	return strings.Contains(DetectContentType(data), "text/")
}

// IsImageFile detects if data is an image format
func IsImageFile(data []byte) bool {
	return strings.Contains(DetectContentType(data), "image/")
}

// IsSVGImageFile detects if data is an SVG image format
func IsSVGImageFile(data []byte) bool {
	return strings.Contains(DetectContentType(data), SVGMimeType)
}

// IsPDFFile detects if data is a pdf format
func IsPDFFile(data []byte) bool {
	return strings.Contains(DetectContentType(data), "application/pdf")
}

// IsVideoFile detects if data is an video format
func IsVideoFile(data []byte) bool {
	return strings.Contains(DetectContentType(data), "video/")
}

// IsAudioFile detects if data is an video format
func IsAudioFile(data []byte) bool {
	return strings.Contains(DetectContentType(data), "audio/")
}

// EntryIcon returns the octicon class for displaying files/directories
func EntryIcon(entry *git.TreeEntry) string {
	switch {
	case entry.IsLink():
		te, err := entry.FollowLink()
		if err != nil {
			log.Debug(err.Error())
			return "file-symlink-file"
		}
		if te.IsDir() {
			return "file-submodule"
		}
		return "file-symlink-file"
	case entry.IsDir():
		return "file-directory"
	case entry.IsSubModule():
		return "file-submodule"
	}

	return "file"
}

// SetupGiteaRoot Sets GITEA_ROOT if it is not already set and returns the value
func SetupGiteaRoot() string {
	giteaRoot := os.Getenv("GITEA_ROOT")
	if giteaRoot == "" {
		_, filename, _, _ := runtime.Caller(0)
		giteaRoot = strings.TrimSuffix(filename, "modules/base/tool.go")
		wd, err := os.Getwd()
		if err != nil {
			rel, err := filepath.Rel(giteaRoot, wd)
			if err != nil && strings.HasPrefix(filepath.ToSlash(rel), "../") {
				giteaRoot = wd
			}
		}
		if _, err := os.Stat(filepath.Join(giteaRoot, "gitea")); os.IsNotExist(err) {
			giteaRoot = ""
		} else if err := os.Setenv("GITEA_ROOT", giteaRoot); err != nil {
			giteaRoot = ""
		}
	}
	return giteaRoot
}

// FormatNumberSI format a number
func FormatNumberSI(data interface{}) string {
	var num int64
	if num1, ok := data.(int64); ok {
		num = num1
	} else if num1, ok := data.(int); ok {
		num = int64(num1)
	} else {
		return ""
	}

	if num < 1000 {
		return fmt.Sprintf("%d", num)
	} else if num < 1000000 {
		num2 := float32(num) / float32(1000.0)
		return fmt.Sprintf("%.1fk", num2)
	} else if num < 1000000000 {
		num2 := float32(num) / float32(1000000.0)
		return fmt.Sprintf("%.1fM", num2)
	}
	num2 := float32(num) / float32(1000000000.0)
	return fmt.Sprintf("%.1fG", num2)
}
