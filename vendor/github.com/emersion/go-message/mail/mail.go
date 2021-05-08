// Package mail implements reading and writing mail messages.
//
// This package assumes that a mail message contains one or more text parts and
// zero or more attachment parts. Each text part represents a different version
// of the message content (e.g. a different type, a different language and so
// on).
//
// RFC 5322 defines the Internet Message Format.
package mail
