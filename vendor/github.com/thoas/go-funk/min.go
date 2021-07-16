package funk

import "strings"

// MinInt validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []int
// It returns int or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinInt(i []int) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min int
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinInt8 validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []int8
// It returns int8 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinInt8(i []int8) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min int8
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinInt16 validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []int16
// It returns int16 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinInt16(i []int16) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min int16
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinInt32 validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []int32
// It returns int32 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinInt32(i []int32) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min int32
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinInt64 validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []int64
// It returns int64 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinInt64(i []int64) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min int64
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinFloat32 validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []float32
// It returns float32 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinFloat32(i []float32) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min float32
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinFloat64 validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []float64
// It returns float64 or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinFloat64(i []float64) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min float64
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		if item < min {
			min = item
		}
	}
	return min
}

// MinString validates the input, compares the elements and returns the minimum element in an array/slice.
// It accepts []string
// It returns string or nil
// It returns nil for the following cases:
//  - input is of length 0
func MinString(i []string) interface{} {
	if len(i) == 0 {
		return nil
	}
	var min string
	for idx := 0; idx < len(i); idx++ {
		item := i[idx]
		if idx == 0 {
			min = item
			continue
		}
		min = compareStringsMin(min, item)
	}
	return min
}

func compareStringsMin(min, current string) string {
	r := strings.Compare(strings.ToLower(min), strings.ToLower(current))
	if r < 0 {
		return min
	}
	return current
}