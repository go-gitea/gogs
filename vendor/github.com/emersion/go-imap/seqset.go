package imap

import (
	"fmt"
	"strconv"
	"strings"
)

// ErrBadSeqSet is used to report problems with the format of a sequence set
// value.
type ErrBadSeqSet string

func (err ErrBadSeqSet) Error() string {
	return fmt.Sprintf("imap: bad sequence set value %q", string(err))
}

// Seq represents a single seq-number or seq-range value (RFC 3501 ABNF). Values
// may be static (e.g. "1", "2:4") or dynamic (e.g. "*", "1:*"). A seq-number is
// represented by setting Start = Stop. Zero is used to represent "*", which is
// safe because seq-number uses nz-number rule. The order of values is always
// Start <= Stop, except when representing "n:*", where Start = n and Stop = 0.
type Seq struct {
	Start, Stop uint32
}

// parseSeqNumber parses a single seq-number value (non-zero uint32 or "*").
func parseSeqNumber(v string) (uint32, error) {
	if n, err := strconv.ParseUint(v, 10, 32); err == nil && v[0] != '0' {
		return uint32(n), nil
	} else if v == "*" {
		return 0, nil
	}
	return 0, ErrBadSeqSet(v)
}

// parseSeq creates a new seq instance by parsing strings in the format "n" or
// "n:m", where n and/or m may be "*". An error is returned for invalid values.
func parseSeq(v string) (s Seq, err error) {
	if sep := strings.IndexRune(v, ':'); sep < 0 {
		s.Start, err = parseSeqNumber(v)
		s.Stop = s.Start
		return
	} else if s.Start, err = parseSeqNumber(v[:sep]); err == nil {
		if s.Stop, err = parseSeqNumber(v[sep+1:]); err == nil {
			if (s.Stop < s.Start && s.Stop != 0) || s.Start == 0 {
				s.Start, s.Stop = s.Stop, s.Start
			}
			return
		}
	}
	return s, ErrBadSeqSet(v)
}

// Contains returns true if the seq-number q is contained in sequence value s.
// The dynamic value "*" contains only other "*" values, the dynamic range "n:*"
// contains "*" and all numbers >= n.
func (s Seq) Contains(q uint32) bool {
	if q == 0 {
		return s.Stop == 0 // "*" is contained only in "*" and "n:*"
	}
	return s.Start != 0 && s.Start <= q && (q <= s.Stop || s.Stop == 0)
}

// Less returns true if s precedes and does not contain seq-number q.
func (s Seq) Less(q uint32) bool {
	return (s.Stop < q || q == 0) && s.Stop != 0
}

// Merge combines sequence values s and t into a single union if the two
// intersect or one is a superset of the other. The order of s and t does not
// matter. If the values cannot be merged, s is returned unmodified and ok is
// set to false.
func (s Seq) Merge(t Seq) (union Seq, ok bool) {
	if union = s; s == t {
		ok = true
		return
	}
	if s.Start != 0 && t.Start != 0 {
		// s and t are any combination of "n", "n:m", or "n:*"
		if s.Start > t.Start {
			s, t = t, s
		}
		// s starts at or before t, check where it ends
		if (s.Stop >= t.Stop && t.Stop != 0) || s.Stop == 0 {
			return s, true // s is a superset of t
		}
		// s is "n" or "n:m", if m == ^uint32(0) then t is "n:*"
		if s.Stop+1 >= t.Start || s.Stop == ^uint32(0) {
			return Seq{s.Start, t.Stop}, true // s intersects or touches t
		}
		return
	}
	// exactly one of s and t is "*"
	if s.Start == 0 {
		if t.Stop == 0 {
			return t, true // s is "*", t is "n:*"
		}
	} else if s.Stop == 0 {
		return s, true // s is "n:*", t is "*"
	}
	return
}

// String returns sequence value s as a seq-number or seq-range string.
func (s Seq) String() string {
	if s.Start == s.Stop {
		if s.Start == 0 {
			return "*"
		}
		return strconv.FormatUint(uint64(s.Start), 10)
	}
	b := strconv.AppendUint(make([]byte, 0, 24), uint64(s.Start), 10)
	if s.Stop == 0 {
		return string(append(b, ':', '*'))
	}
	return string(strconv.AppendUint(append(b, ':'), uint64(s.Stop), 10))
}

// SeqSet is used to represent a set of message sequence numbers or UIDs (see
// sequence-set ABNF rule). The zero value is an empty set.
type SeqSet struct {
	Set []Seq
}

// ParseSeqSet returns a new SeqSet instance after parsing the set string.
func ParseSeqSet(set string) (s *SeqSet, err error) {
	s = new(SeqSet)
	return s, s.Add(set)
}

// Add inserts new sequence values into the set. The string format is described
// by RFC 3501 sequence-set ABNF rule. If an error is encountered, all values
// inserted successfully prior to the error remain in the set.
func (s *SeqSet) Add(set string) error {
	for _, sv := range strings.Split(set, ",") {
		v, err := parseSeq(sv)
		if err != nil {
			return err
		}
		s.insert(v)
	}
	return nil
}

// AddNum inserts new sequence numbers into the set. The value 0 represents "*".
func (s *SeqSet) AddNum(q ...uint32) {
	for _, v := range q {
		s.insert(Seq{v, v})
	}
}

// AddRange inserts a new sequence range into the set.
func (s *SeqSet) AddRange(Start, Stop uint32) {
	if (Stop < Start && Stop != 0) || Start == 0 {
		s.insert(Seq{Stop, Start})
	} else {
		s.insert(Seq{Start, Stop})
	}
}

// AddSet inserts all values from t into s.
func (s *SeqSet) AddSet(t *SeqSet) {
	for _, v := range t.Set {
		s.insert(v)
	}
}

// Clear removes all values from the set.
func (s *SeqSet) Clear() {
	s.Set = s.Set[:0]
}

// Empty returns true if the sequence set does not contain any values.
func (s SeqSet) Empty() bool {
	return len(s.Set) == 0
}

// Dynamic returns true if the set contains "*" or "n:*" values.
func (s SeqSet) Dynamic() bool {
	return len(s.Set) > 0 && s.Set[len(s.Set)-1].Stop == 0
}

// Contains returns true if the non-zero sequence number or UID q is contained
// in the set. The dynamic range "n:*" contains all q >= n. It is the caller's
// responsibility to handle the special case where q is the maximum UID in the
// mailbox and q < n (i.e. the set cannot match UIDs against "*:n" or "*" since
// it doesn't know what the maximum value is).
func (s SeqSet) Contains(q uint32) bool {
	if _, ok := s.search(q); ok {
		return q != 0
	}
	return false
}

// String returns a sorted representation of all contained sequence values.
func (s SeqSet) String() string {
	if len(s.Set) == 0 {
		return ""
	}
	b := make([]byte, 0, 64)
	for _, v := range s.Set {
		b = append(b, ',')
		if v.Start == 0 {
			b = append(b, '*')
			continue
		}
		b = strconv.AppendUint(b, uint64(v.Start), 10)
		if v.Start != v.Stop {
			if v.Stop == 0 {
				b = append(b, ':', '*')
				continue
			}
			b = strconv.AppendUint(append(b, ':'), uint64(v.Stop), 10)
		}
	}
	return string(b[1:])
}

// insert adds sequence value v to the set.
func (s *SeqSet) insert(v Seq) {
	i, _ := s.search(v.Start)
	merged := false
	if i > 0 {
		// try merging with the preceding entry (e.g. "1,4".insert(2), i == 1)
		s.Set[i-1], merged = s.Set[i-1].Merge(v)
	}
	if i == len(s.Set) {
		// v was either merged with the last entry or needs to be appended
		if !merged {
			s.insertAt(i, v)
		}
		return
	} else if merged {
		i--
	} else if s.Set[i], merged = s.Set[i].Merge(v); !merged {
		s.insertAt(i, v) // insert in the middle (e.g. "1,5".insert(3), i == 1)
		return
	}
	// v was merged with s.Set[i], continue trying to merge until the end
	for j := i + 1; j < len(s.Set); j++ {
		if s.Set[i], merged = s.Set[i].Merge(s.Set[j]); !merged {
			if j > i+1 {
				// cut out all entries between i and j that were merged
				s.Set = append(s.Set[:i+1], s.Set[j:]...)
			}
			return
		}
	}
	// everything after s.Set[i] was merged
	s.Set = s.Set[:i+1]
}

// insertAt inserts a new sequence value v at index i, resizing s.Set as needed.
func (s *SeqSet) insertAt(i int, v Seq) {
	if n := len(s.Set); i == n {
		// insert at the end
		s.Set = append(s.Set, v)
		return
	} else if n < cap(s.Set) {
		// enough space, shift everything at and after i to the right
		s.Set = s.Set[:n+1]
		copy(s.Set[i+1:], s.Set[i:])
	} else {
		// allocate new slice and copy everything, n is at least 1
		set := make([]Seq, n+1, n*2)
		copy(set, s.Set[:i])
		copy(set[i+1:], s.Set[i:])
		s.Set = set
	}
	s.Set[i] = v
}

// search attempts to find the index of the sequence set value that contains q.
// If no values contain q, the returned index is the position where q should be
// inserted and ok is set to false.
func (s SeqSet) search(q uint32) (i int, ok bool) {
	min, max := 0, len(s.Set)-1
	for min < max {
		if mid := (min + max) >> 1; s.Set[mid].Less(q) {
			min = mid + 1
		} else {
			max = mid
		}
	}
	if max < 0 || s.Set[min].Less(q) {
		return len(s.Set), false // q is the new largest value
	}
	return min, s.Set[min].Contains(q)
}
