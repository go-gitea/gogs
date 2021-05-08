package imap

import (
	"fmt"
	"regexp"
	"time"
)

// Date and time layouts.
// Dovecot adds a leading zero to dates:
//   https://github.com/dovecot/core/blob/4fbd5c5e113078e72f29465ccc96d44955ceadc2/src/lib-imap/imap-date.c#L166
// Cyrus adds a leading space to dates:
//   https://github.com/cyrusimap/cyrus-imapd/blob/1cb805a3bffbdf829df0964f3b802cdc917e76db/lib/times.c#L543
// GMail doesn't support leading spaces in dates used in SEARCH commands.
const (
	// Defined in RFC 3501 as date-text on page 83.
	DateLayout = "_2-Jan-2006"
	// Defined in RFC 3501 as date-time on page 83.
	DateTimeLayout = "_2-Jan-2006 15:04:05 -0700"
	// Defined in RFC 5322 section 3.3, mentioned as env-date in RFC 3501 page 84.
	envelopeDateTimeLayout = "Mon, 02 Jan 2006 15:04:05 -0700"
	// Use as an example in RFC 3501 page 54.
	searchDateLayout = "2-Jan-2006"
)

// time.Time with a specific layout.
type (
	Date             time.Time
	DateTime         time.Time
	envelopeDateTime time.Time
	searchDate       time.Time
)

// Permutations of the layouts defined in RFC 5322, section 3.3.
var envelopeDateTimeLayouts = [...]string{
	envelopeDateTimeLayout, // popular, try it first
	"_2 Jan 2006 15:04:05 -0700",
	"_2 Jan 2006 15:04:05 MST",
	"_2 Jan 2006 15:04 -0700",
	"_2 Jan 2006 15:04 MST",
	"_2 Jan 06 15:04:05 -0700",
	"_2 Jan 06 15:04:05 MST",
	"_2 Jan 06 15:04 -0700",
	"_2 Jan 06 15:04 MST",
	"Mon, _2 Jan 2006 15:04:05 -0700",
	"Mon, _2 Jan 2006 15:04:05 MST",
	"Mon, _2 Jan 2006 15:04 -0700",
	"Mon, _2 Jan 2006 15:04 MST",
	"Mon, _2 Jan 06 15:04:05 -0700",
	"Mon, _2 Jan 06 15:04:05 MST",
	"Mon, _2 Jan 06 15:04 -0700",
	"Mon, _2 Jan 06 15:04 MST",
}

// TODO: this is a blunt way to strip any trailing CFWS (comment). A sharper
// one would strip multiple CFWS, and only if really valid according to
// RFC5322.
var commentRE = regexp.MustCompile(`[ \t]+\(.*\)$`)

// Try parsing the date based on the layouts defined in RFC 5322, section 3.3.
// Inspired by https://github.com/golang/go/blob/master/src/net/mail/message.go
func parseMessageDateTime(maybeDate string) (time.Time, error) {
	maybeDate = commentRE.ReplaceAllString(maybeDate, "")
	for _, layout := range envelopeDateTimeLayouts {
		parsed, err := time.Parse(layout, maybeDate)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("date %s could not be parsed", maybeDate)
}
