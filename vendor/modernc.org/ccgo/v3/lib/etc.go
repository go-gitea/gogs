// Copyright 2020 The CCGO Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ccgo // import "modernc.org/ccgo/v3/lib"

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	"modernc.org/cc/v3"
)

var (
	reservedNames = map[string]bool{
		"bool":        false, // ccgo can use
		"break":       true,  // keyword
		"case":        true,  // keyword
		"chan":        true,  // keyword
		"const":       true,  // keyword
		"continue":    true,  // keyword
		"default":     true,  // keyword
		"defer":       true,  // keyword
		"else":        true,  // keyword
		"fallthrough": true,  // keyword
		"false":       false, // ccgo can use
		"float32":     false, // ccgo can use
		"float64":     false, // ccgo can use
		"for":         true,  // keyword
		"func":        true,  // keyword
		"go":          true,  // keyword
		"goto":        true,  // keyword
		"if":          true,  // keyword
		"import":      true,  // keyword
		"init":        false, // special name
		"int16":       false, // ccgo can use
		"int32":       false, // ccgo can use
		"int64":       false, // ccgo can use
		"int8":        false, // ccgo can use
		"interface":   true,  // keyword
		"map":         true,  // keyword
		"math":        false, // package name
		"nil":         false, // ccgo can use
		"package":     true,  // keyword
		"range":       true,  // keyword
		"return":      true,  // keyword
		"select":      true,  // keyword
		"struct":      true,  // keyword
		"switch":      true,  // keyword
		"true":        false, // ccgo can use
		"type":        true,  // keyword
		"types":       false, // package name
		"uint16":      false, // ccgo can use
		"uint32":      false, // ccgo can use
		"uint64":      false, // ccgo can use
		"uint8":       false, // ccgo can use
		"uintptr":     false, // ccgo can use
		"unsafe":      false, // package name
		"var":         true,  // keyword
	}

	reservedIds []cc.StringID

	maxInt32  = big.NewInt(math.MaxInt32)
	maxInt64  = big.NewInt(math.MaxInt64)
	maxUint32 = big.NewInt(math.MaxUint32)
	maxUint64 = big.NewInt(0).SetUint64(math.MaxUint64)
	minInt32  = big.NewInt(math.MinInt32)
	minInt64  = big.NewInt(math.MinInt64)
)

func init() {
	for k := range reservedNames {
		reservedIds = append(reservedIds, cc.String(k))
	}
}

type scope map[cc.StringID]int32

func newScope() scope {
	s := scope{}
	for _, k := range reservedIds {
		s[k] = 0
	}
	return s
}

func (s scope) take(t cc.StringID) string {
	if t == 0 {
		panic(todo("internal error"))
	}

	n, ok := s[t]
	if !ok {
		s[t] = 0
		return t.String()
	}

	for {
		n++
		s[t] = n
		r := fmt.Sprintf("%s%d", t, n)
		id := cc.String(r)
		if _, ok := s[id]; !ok {
			s[id] = 0
			return r
		}
	}
}

func dumpLayout(t cc.Type, info *structInfo) string {
	switch t.Kind() {
	case cc.Struct, cc.Union:
		// ok
	default:
		return t.String()
	}

	nf := t.NumField()
	var a []string
	w := 0
	for i := 0; i < nf; i++ {
		if n := len(t.FieldByIndex([]int{i}).Name().String()); n > w {
			w = n
		}
	}
	for i := 0; i < nf; i++ {
		f := t.FieldByIndex([]int{i})
		var bf cc.StringID
		if f.IsBitField() {
			if bfbf := f.BitFieldBlockFirst(); bfbf != nil {
				bf = bfbf.Name()
			}
		}
		a = append(a, fmt.Sprintf("%3d: %*q: BitFieldOffset %3v, BitFieldWidth %3v, IsBitField %5v, Mask: %#016x, off: %3v, pad %2v, BitFieldBlockWidth: %2d, BitFieldBlockFirst: %s, %v",
			i, w+2, f.Name(), f.BitFieldOffset(), f.BitFieldWidth(),
			f.IsBitField(), f.Mask(), f.Offset(), f.Padding(),
			f.BitFieldBlockWidth(), bf, f.Type(),
		))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%v\n%s\n----\n", t, strings.Join(a, "\n"))
	fmt.Fprintf(&b, "offs: %v\n", info.offs)
	a = a[:0]
	for k, v := range info.flds {
		var b []string
		for _, w := range v {
			b = append(b, fmt.Sprintf("%q padBefore: %d ", w.Name(), info.padBefore[w]))
		}
		a = append(a, fmt.Sprintf("%4d %s", k, b))
	}
	sort.Strings(a)
	for _, v := range a {
		fmt.Fprintf(&b, "%s\n", v)
	}
	fmt.Fprintf(&b, "padAfter: %v\n", info.padAfter)
	return b.String()
}
