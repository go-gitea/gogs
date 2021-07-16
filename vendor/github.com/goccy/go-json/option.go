package json

import (
	"github.com/goccy/go-json/internal/decoder"
	"github.com/goccy/go-json/internal/encoder"
)

type EncodeOption = encoder.Option
type EncodeOptionFunc func(*EncodeOption)

// UnorderedMap doesn't sort when encoding map type.
func UnorderedMap() EncodeOptionFunc {
	return func(opt *EncodeOption) {
		opt.Flag |= encoder.UnorderedMapOption
	}
}

// Debug outputs debug information when panic occurs during encoding.
func Debug() EncodeOptionFunc {
	return func(opt *EncodeOption) {
		opt.Flag |= encoder.DebugOption
	}
}

// Colorize add an identifier for coloring to the string of the encoded result.
func Colorize(scheme *ColorScheme) EncodeOptionFunc {
	return func(opt *EncodeOption) {
		opt.Flag |= encoder.ColorizeOption
		opt.ColorScheme = scheme
	}
}

type DecodeOption = decoder.Option
type DecodeOptionFunc func(*DecodeOption)

// DecodeFieldPriorityFirstWin
// in the default behavior, go-json, like encoding/json,
// will reflect the result of the last evaluation when a field with the same name exists.
// This option allow you to change this behavior.
// this option reflects the result of the first evaluation if a field with the same name exists.
// This behavior has a performance advantage as it allows the subsequent strings to be skipped if all fields have been evaluated.
func DecodeFieldPriorityFirstWin() DecodeOptionFunc {
	return func(opt *DecodeOption) {
		opt.Flags |= decoder.FirstWinOption
	}
}
