package funk

import "strings"

// MaxInt validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []int
// It returns int or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxInt(i []int) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max int
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxInt8 validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []int8
// It returns int8 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxInt8(i []int8) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max int8
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxInt16 validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []int16
// It returns int16 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxInt16(i []int16) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max int16
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxInt32 validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []int32
// It returns int32 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxInt32(i []int32) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max int32
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxInt64 validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []int64
// It returns int64 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxInt64(i []int64) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max int64
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxFloat32 validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []float32
// It returns float32 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxFloat32(i []float32) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max float32
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxFloat64 validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []float64
// It returns float64 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxFloat64(i []float64) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max float64
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		if item > max {
			max = item
		}
	}
	return max
}

// MaxString validates the input, compares the elements and returns the maximum element in an array/slice.
// It accepts []string
// It returns string or nil
// It returns nil for the following cases:
//  - input is of length 0
func MaxString(i []string) interface{} {
	if len(i) == 0 {
		return nil
	}
	var max string
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			max = item
			continue
		}
		max = compareStringsMax(max, item)
	}
	return max
}

// compareStrings uses the strings.Compare method to compare two strings, and returns the greater one.
func compareStringsMax(max, current string) string {
	r := strings.Compare(strings.ToLower(max), strings.ToLower(current))
	if r > 0 {
		return max
	}
	return current
}