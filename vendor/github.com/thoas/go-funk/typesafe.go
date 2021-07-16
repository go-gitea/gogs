package funk

import (
	"math/rand"
)

// InInts is an alias of ContainsInt, returns true if an int is present in a iteratee.
func InInts(s []int, v int) bool {
	return ContainsInt(s, v)
}

// InInt32s is an alias of ContainsInt32, returns true if an int32 is present in a iteratee.
func InInt32s(s []int32, v int32) bool {
	return ContainsInt32(s, v)
}

// InInt64s is an alias of ContainsInt64, returns true if an int64 is present in a iteratee.
func InInt64s(s []int64, v int64) bool {
	return ContainsInt64(s, v)
}

// InUInts is an alias of ContainsUInt, returns true if an uint is present in a iteratee.
func InUInts(s []uint, v uint) bool {
	return ContainsUInt(s, v)
}

// InUInt32s is an alias of ContainsUInt32, returns true if an uint32 is present in a iteratee.
func InUInt32s(s []uint32, v uint32) bool {
	return ContainsUInt32(s, v)
}

// InUInt64s is an alias of ContainsUInt64, returns true if an uint64 is present in a iteratee.
func InUInt64s(s []uint64, v uint64) bool {
	return ContainsUInt64(s, v)
}

// InStrings is an alias of ContainsString, returns true if a string is present in a iteratee.
func InStrings(s []string, v string) bool {
	return ContainsString(s, v)
}

// InFloat32s is an alias of ContainsFloat32, returns true if a float32 is present in a iteratee.
func InFloat32s(s []float32, v float32) bool {
	return ContainsFloat32(s, v)
}

// InFloat64s is an alias of ContainsFloat64, returns true if a float64 is present in a iteratee.
func InFloat64s(s []float64, v float64) bool {
	return ContainsFloat64(s, v)
}

// FindFloat64 iterates over a collection of float64, returning an array of
// all float64 elements predicate returns truthy for.
func FindFloat64(s []float64, cb func(s float64) bool) (float64, bool) {
	for _, i := range s {
		result := cb(i)

		if result {
			return i, true
		}
	}

	return 0.0, false
}

// FindFloat32 iterates over a collection of float32, returning the first
// float32 element predicate returns truthy for.
func FindFloat32(s []float32, cb func(s float32) bool) (float32, bool) {
	for _, i := range s {
		result := cb(i)

		if result {
			return i, true
		}
	}

	return 0.0, false
}

// FindInt iterates over a collection of int, returning the first
// int element predicate returns truthy for.
func FindInt(s []int, cb func(s int) bool) (int, bool) {
	for _, i := range s {
		result := cb(i)

		if result {
			return i, true
		}
	}

	return 0, false
}

// FindInt32 iterates over a collection of int32, returning the first
// int32 element predicate returns truthy for.
func FindInt32(s []int32, cb func(s int32) bool) (int32, bool) {
	for _, i := range s {
		result := cb(i)

		if result {
			return i, true
		}
	}

	return 0, false
}

// FindInt64 iterates over a collection of int64, returning the first
// int64 element predicate returns truthy for.
func FindInt64(s []int64, cb func(s int64) bool) (int64, bool) {
	for _, i := range s {
		result := cb(i)

		if result {
			return i, true
		}
	}

	return 0, false
}

// FindString iterates over a collection of string, returning the first
// string element predicate returns truthy for.
func FindString(s []string, cb func(s string) bool) (string, bool) {
	for _, i := range s {
		result := cb(i)

		if result {
			return i, true
		}
	}

	return "", false
}

// FilterFloat64 iterates over a collection of float64, returning an array of
// all float64 elements predicate returns truthy for.
func FilterFloat64(s []float64, cb func(s float64) bool) []float64 {
	results := []float64{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterFloat32 iterates over a collection of float32, returning an array of
// all float32 elements predicate returns truthy for.
func FilterFloat32(s []float32, cb func(s float32) bool) []float32 {
	results := []float32{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterInt iterates over a collection of int, returning an array of
// all int elements predicate returns truthy for.
func FilterInt(s []int, cb func(s int) bool) []int {
	results := []int{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterInt32 iterates over a collection of int32, returning an array of
// all int32 elements predicate returns truthy for.
func FilterInt32(s []int32, cb func(s int32) bool) []int32 {
	results := []int32{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterInt64 iterates over a collection of int64, returning an array of
// all int64 elements predicate returns truthy for.
func FilterInt64(s []int64, cb func(s int64) bool) []int64 {
	results := []int64{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterUInt iterates over a collection of uint, returning an array of
// all uint elements predicate returns truthy for.
func FilterUInt(s []uint, cb func(s uint) bool) []uint {
	results := []uint{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterUInt32 iterates over a collection of uint32, returning an array of
// all uint32 elements predicate returns truthy for.
func FilterUInt32(s []uint32, cb func(s uint32) bool) []uint32 {
	results := []uint32{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterUInt64 iterates over a collection of uint64, returning an array of
// all uint64 elements predicate returns truthy for.
func FilterUInt64(s []uint64, cb func(s uint64) bool) []uint64 {
	results := []uint64{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// FilterString iterates over a collection of string, returning an array of
// all string elements predicate returns truthy for.
func FilterString(s []string, cb func(s string) bool) []string {
	results := []string{}

	for _, i := range s {
		result := cb(i)

		if result {
			results = append(results, i)
		}
	}

	return results
}

// ContainsInt returns true if an int is present in a iteratee.
func ContainsInt(s []int, v int) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsInt32 returns true if an int32 is present in a iteratee.
func ContainsInt32(s []int32, v int32) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsInt64 returns true if an int64 is present in a iteratee.
func ContainsInt64(s []int64, v int64) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsUInt returns true if an uint is present in a iteratee.
func ContainsUInt(s []uint, v uint) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsUInt32 returns true if an uint32 is present in a iteratee.
func ContainsUInt32(s []uint32, v uint32) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsUInt64 returns true if an uint64 is present in a iteratee.
func ContainsUInt64(s []uint64, v uint64) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}


// ContainsString returns true if a string is present in a iteratee.
func ContainsString(s []string, v string) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsFloat32 returns true if a float32 is present in a iteratee.
func ContainsFloat32(s []float32, v float32) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// ContainsFloat64 returns true if a float64 is present in a iteratee.
func ContainsFloat64(s []float64, v float64) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// SumInt32 sums a int32 iteratee and returns the sum of all elements
func SumInt32(s []int32) (sum int32) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumInt64 sums a int64 iteratee and returns the sum of all elements
func SumInt64(s []int64) (sum int64) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumInt sums a int iteratee and returns the sum of all elements
func SumInt(s []int) (sum int) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumUInt32 sums a uint32 iteratee and returns the sum of all elements
func SumUInt32(s []uint32) (sum uint32) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumUInt64 sums a uint64 iteratee and returns the sum of all elements
func SumUInt64(s []uint64) (sum uint64) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumUInt sums a uint iteratee and returns the sum of all elements
func SumUInt(s []uint) (sum uint) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumFloat64 sums a float64 iteratee and returns the sum of all elements
func SumFloat64(s []float64) (sum float64) {
	for _, v := range s {
		sum += v
	}
	return
}

// SumFloat32 sums a float32 iteratee and returns the sum of all elements
func SumFloat32(s []float32) (sum float32) {
	for _, v := range s {
		sum += v
	}
	return
}

// ReverseStrings reverses an array of string
func ReverseStrings(s []string) []string {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseInt reverses an array of int
func ReverseInt(s []int) []int {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseInt32 reverses an array of int32
func ReverseInt32(s []int32) []int32 {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseInt64 reverses an array of int64
func ReverseInt64(s []int64) []int64 {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseUInt reverses an array of int
func ReverseUInt(s []uint) []uint {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseUInt32 reverses an array of uint32
func ReverseUInt32(s []uint32) []uint32 {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseUInt64 reverses an array of uint64
func ReverseUInt64(s []uint64) []uint64 {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseFloat64 reverses an array of float64
func ReverseFloat64(s []float64) []float64 {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseFloat32 reverses an array of float32
func ReverseFloat32(s []float32) []float32 {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// ReverseString reverses a string
func ReverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func indexOf(n int, f func(int) bool) int {
	for i := 0; i < n; i++ {
		if f(i) {
			return i
		}
	}
	return -1
}

// IndexOfInt gets the index at which the first occurrence of an int value is found in array or return -1
// if the value cannot be found
func IndexOfInt(a []int, x int) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfInt32 gets the index at which the first occurrence of an int32 value is found in array or return -1
// if the value cannot be found
func IndexOfInt32(a []int32, x int32) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfInt64 gets the index at which the first occurrence of an int64 value is found in array or return -1
// if the value cannot be found
func IndexOfInt64(a []int64, x int64) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfUInt gets the index at which the first occurrence of an uint value is found in array or return -1
// if the value cannot be found
func IndexOfUInt(a []uint, x uint) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfUInt32 gets the index at which the first occurrence of an uint32 value is found in array or return -1
// if the value cannot be found
func IndexOfUInt32(a []uint32, x uint32) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfUInt64 gets the index at which the first occurrence of an uint64 value is found in array or return -1
// if the value cannot be found
func IndexOfUInt64(a []uint64, x uint64) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfFloat64 gets the index at which the first occurrence of an float64 value is found in array or return -1
// if the value cannot be found
func IndexOfFloat64(a []float64, x float64) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

// IndexOfString gets the index at which the first occurrence of a string value is found in array or return -1
// if the value cannot be found
func IndexOfString(a []string, x string) int {
	return indexOf(len(a), func(i int) bool { return a[i] == x })
}

func lastIndexOf(n int, f func(int) bool) int {
	for i := n - 1; i >= 0; i-- {
		if f(i) {
			return i
		}
	}
	return -1
}

// LastIndexOfInt gets the index at which the first occurrence of an int value is found in array or return -1
// if the value cannot be found
func LastIndexOfInt(a []int, x int) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfInt32 gets the index at which the first occurrence of an int32 value is found in array or return -1
// if the value cannot be found
func LastIndexOfInt32(a []int32, x int32) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfInt64 gets the index at which the first occurrence of an int64 value is found in array or return -1
// if the value cannot be found
func LastIndexOfInt64(a []int64, x int64) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfUInt gets the index at which the first occurrence of an uint value is found in array or return -1
// if the value cannot be found
func LastIndexOfUInt(a []uint, x uint) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfUInt32 gets the index at which the first occurrence of an uint32 value is found in array or return -1
// if the value cannot be found
func LastIndexOfUInt32(a []uint32, x uint32) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfUInt64 gets the index at which the first occurrence of an uint64 value is found in array or return -1
// if the value cannot be found
func LastIndexOfUInt64(a []uint64, x uint64) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfFloat64 gets the index at which the first occurrence of an float64 value is found in array or return -1
// if the value cannot be found
func LastIndexOfFloat64(a []float64, x float64) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfFloat32 gets the index at which the first occurrence of an float32 value is found in array or return -1
// if the value cannot be found
func LastIndexOfFloat32(a []float32, x float32) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// LastIndexOfString gets the index at which the first occurrence of a string value is found in array or return -1
// if the value cannot be found
func LastIndexOfString(a []string, x string) int {
	return lastIndexOf(len(a), func(i int) bool { return a[i] == x })
}

// UniqInt32 creates an array of int32 with unique values.
func UniqInt32(a []int32) []int32 {
	length := len(a)

	seen := make(map[int32]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqInt64 creates an array of int64 with unique values.
func UniqInt64(a []int64) []int64 {
	length := len(a)

	seen := make(map[int64]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqInt creates an array of int with unique values.
func UniqInt(a []int) []int {
	length := len(a)

	seen := make(map[int]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqUInt32 creates an array of uint32 with unique values.
func UniqUInt32(a []uint32) []uint32 {
	length := len(a)

	seen := make(map[uint32]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqUInt64 creates an array of uint64 with unique values.
func UniqUInt64(a []uint64) []uint64 {
	length := len(a)

	seen := make(map[uint64]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqUInt creates an array of uint with unique values.
func UniqUInt(a []uint) []uint {
	length := len(a)

	seen := make(map[uint]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqString creates an array of string with unique values.
func UniqString(a []string) []string {
	length := len(a)

	seen := make(map[string]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqFloat64 creates an array of float64 with unique values.
func UniqFloat64(a []float64) []float64 {
	length := len(a)

	seen := make(map[float64]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// UniqFloat32 creates an array of float32 with unique values.
func UniqFloat32(a []float32) []float32 {
	length := len(a)

	seen := make(map[float32]struct{}, length)
	j := 0

	for i := 0; i < length; i++ {
		v := a[i]

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		a[j] = v
		j++
	}

	return a[0:j]
}

// ShuffleInt creates an array of int shuffled values using Fisher–Yates algorithm
func ShuffleInt(a []int) []int {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleInt32 creates an array of int32 shuffled values using Fisher–Yates algorithm
func ShuffleInt32(a []int32) []int32 {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleInt64 creates an array of int64 shuffled values using Fisher–Yates algorithm
func ShuffleInt64(a []int64) []int64 {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleUInt creates an array of int shuffled values using Fisher–Yates algorithm
func ShuffleUInt(a []uint) []uint {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleUInt32 creates an array of uint32 shuffled values using Fisher–Yates algorithm
func ShuffleUInt32(a []uint32) []uint32 {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleUInt64 creates an array of uint64 shuffled values using Fisher–Yates algorithm
func ShuffleUInt64(a []uint64) []uint64 {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleString creates an array of string shuffled values using Fisher–Yates algorithm
func ShuffleString(a []string) []string {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleFloat32 creates an array of float32 shuffled values using Fisher–Yates algorithm
func ShuffleFloat32(a []float32) []float32 {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// ShuffleFloat64 creates an array of float64 shuffled values using Fisher–Yates algorithm
func ShuffleFloat64(a []float64) []float64 {
	for i := range a {
		j := rand.Intn(i + 1)
		a[i], a[j] = a[j], a[i]
	}

	return a
}

// DropString creates a slice with `n` strings dropped from the beginning.
func DropString(s []string, n int) []string {
	return s[n:]
}

// DropInt creates a slice with `n` ints dropped from the beginning.
func DropInt(s []int, n int) []int {
	return s[n:]
}

// DropInt32 creates a slice with `n` int32s dropped from the beginning.
func DropInt32(s []int32, n int) []int32 {
	return s[n:]
}

// DropInt64 creates a slice with `n` int64s dropped from the beginning.
func DropInt64(s []int64, n int) []int64 {
	return s[n:]
}

// DropUInt creates a slice with `n` ints dropped from the beginning.
func DropUInt(s []uint, n uint) []uint {
	return s[n:]
}

// DropUInt32 creates a slice with `n` int32s dropped from the beginning.
func DropUInt32(s []uint32, n int) []uint32 {
	return s[n:]
}

// DropUInt64 creates a slice with `n` int64s dropped from the beginning.
func DropUInt64(s []uint64, n int) []uint64 {
	return s[n:]
}

// DropFloat32 creates a slice with `n` float32s dropped from the beginning.
func DropFloat32(s []float32, n int) []float32 {
	return s[n:]
}

// DropFloat64 creates a slice with `n` float64s dropped from the beginning.
func DropFloat64(s []float64, n int) []float64 {
	return s[n:]
}
