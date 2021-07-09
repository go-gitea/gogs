package ledis

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/siddontang/ledisdb/store"
)

// Limit is for sort.
type Limit struct {
	Offset int
	Size   int
}

func getSortRange(values [][]byte, offset int, size int) (int, int) {
	var start = 0
	if offset > 0 {
		start = offset
	}

	valueLen := len(values)
	var end = valueLen - 1
	if size > 0 {
		end = start + size - 1
	}

	if start >= valueLen {
		start = valueLen - 1
		end = valueLen - 2
	}

	if end >= valueLen {
		end = valueLen - 1
	}

	return start, end
}

var hashPattern = []byte("*->")

func (db *DB) lookupKeyByPattern(pattern []byte, subKey []byte) []byte {
	// If the pattern is #, return the substitution key itself
	if bytes.Equal(pattern, []byte{'#'}) {
		return subKey
	}

	// If we can't find '*' in the pattern, return nil
	if !bytes.Contains(pattern, []byte{'*'}) {
		return nil
	}

	key := pattern
	var field []byte

	// Find out if we're dealing with a hash dereference
	if n := bytes.Index(pattern, hashPattern); n > 0 && n+3 < len(pattern) {
		key = pattern[0 : n+1]
		field = pattern[n+3:]
	}

	// Perform the '*' substitution
	key = bytes.Replace(key, []byte{'*'}, subKey, 1)

	var value []byte
	if field == nil {
		value, _ = db.Get(key)
	} else {
		value, _ = db.HGet(key, field)
	}

	return value
}

type sortItem struct {
	value    []byte
	cmpValue []byte
	score    float64
}

type sortItemSlice struct {
	alpha         bool
	sortByPattern bool
	items         []sortItem
}

func (s *sortItemSlice) Len() int {
	return len(s.items)
}

func (s *sortItemSlice) Swap(i, j int) {
	s.items[i], s.items[j] = s.items[j], s.items[i]
}

func (s *sortItemSlice) Less(i, j int) bool {
	s1 := s.items[i]
	s2 := s.items[j]
	if !s.alpha {
		if s1.score < s2.score {
			return true
		} else if s1.score > s2.score {
			return false
		} else {
			return bytes.Compare(s1.value, s2.value) < 0
		}
	} else {
		if s.sortByPattern {
			if s1.cmpValue == nil || s2.cmpValue == nil {
				if s1.cmpValue == nil {
					return true
				}
				return false
			}
			// Unlike redis, we only use bytes compare
			return bytes.Compare(s1.cmpValue, s2.cmpValue) < 0
		}

		// Unlike redis, we only use bytes compare
		return bytes.Compare(s1.value, s2.value) < 0
	}
}

func (db *DB) xsort(values [][]byte, offset int, size int, alpha bool, desc bool, sortBy []byte, sortGet [][]byte) ([][]byte, error) {
	if len(values) == 0 {
		return [][]byte{}, nil
	}

	start, end := getSortRange(values, offset, size)

	dontsort := 0

	if sortBy != nil {
		if !bytes.Contains(sortBy, []byte{'*'}) {
			dontsort = 1
		}
	}

	items := &sortItemSlice{
		alpha:         alpha,
		sortByPattern: sortBy != nil,
		items:         make([]sortItem, len(values)),
	}

	for i, value := range values {
		items.items[i].value = value
		items.items[i].score = 0
		items.items[i].cmpValue = nil

		if dontsort == 0 {
			var cmpValue []byte
			if sortBy != nil {
				cmpValue = db.lookupKeyByPattern(sortBy, value)
			} else {
				// use value iteself to sort by
				cmpValue = value
			}

			if cmpValue == nil {
				continue
			}

			if alpha {
				if sortBy != nil {
					items.items[i].cmpValue = cmpValue
				}
			} else {
				score, err := strconv.ParseFloat(string(cmpValue), 64)
				if err != nil {
					return nil, fmt.Errorf("%s scores can't be converted into double", cmpValue)
				}
				items.items[i].score = score
			}
		}
	}

	if dontsort == 0 {
		if !desc {
			sort.Sort(items)
		} else {
			sort.Sort(sort.Reverse(items))
		}
	}

	resLen := end - start + 1
	if len(sortGet) > 0 {
		resLen = len(sortGet) * (end - start + 1)
	}

	res := make([][]byte, 0, resLen)
	for i := start; i <= end; i++ {
		if len(sortGet) == 0 {
			res = append(res, items.items[i].value)
		} else {
			for _, getPattern := range sortGet {
				v := db.lookupKeyByPattern(getPattern, items.items[i].value)
				res = append(res, v)
			}
		}
	}

	return res, nil
}

// XLSort sorts list.
func (db *DB) XLSort(key []byte, offset int, size int, alpha bool, desc bool, sortBy []byte, sortGet [][]byte) ([][]byte, error) {
	values, err := db.LRange(key, 0, -1)

	if err != nil {
		return nil, err
	}

	return db.xsort(values, offset, size, alpha, desc, sortBy, sortGet)
}

// XSSort sorts set.
func (db *DB) XSSort(key []byte, offset int, size int, alpha bool, desc bool, sortBy []byte, sortGet [][]byte) ([][]byte, error) {
	values, err := db.SMembers(key)
	if err != nil {
		return nil, err
	}

	return db.xsort(values, offset, size, alpha, desc, sortBy, sortGet)
}

// XZSort sorts zset.
func (db *DB) XZSort(key []byte, offset int, size int, alpha bool, desc bool, sortBy []byte, sortGet [][]byte) ([][]byte, error) {
	values, err := db.ZRangeByLex(key, nil, nil, store.RangeClose, 0, -1)
	if err != nil {
		return nil, err
	}

	return db.xsort(values, offset, size, alpha, desc, sortBy, sortGet)
}
