package funk

import (
	"reflect"
)

// Intersect returns the intersection between two collections.
//
// Deprecated: use Join(x, y, InnerJoin) instead of Intersect, InnerJoin
// implements deduplication mechanism, so verify your code behaviour
// before using it
func Intersect(x interface{}, y interface{}) interface{} {
	if !IsCollection(x) {
		panic("First parameter must be a collection")
	}
	if !IsCollection(y) {
		panic("Second parameter must be a collection")
	}

	hash := map[interface{}]struct{}{}

	xValue := reflect.ValueOf(x)
	xType := xValue.Type()

	yValue := reflect.ValueOf(y)
	yType := yValue.Type()

	if NotEqual(xType, yType) {
		panic("Parameters must have the same type")
	}

	zType := reflect.SliceOf(xType.Elem())
	zSlice := reflect.MakeSlice(zType, 0, 0)

	for i := 0; i < xValue.Len(); i++ {
		v := xValue.Index(i).Interface()
		hash[v] = struct{}{}
	}

	for i := 0; i < yValue.Len(); i++ {
		v := yValue.Index(i).Interface()
		_, ok := hash[v]
		if ok {
			zSlice = reflect.Append(zSlice, yValue.Index(i))
		}
	}

	return zSlice.Interface()
}

// IntersectString returns the intersection between two collections of string.
func IntersectString(x []string, y []string) []string {
	if len(x) == 0 || len(y) == 0 {
		return []string{}
	}

	set := []string{}
	hash := map[string]struct{}{}

	for _, v := range x {
		hash[v] = struct{}{}
	}

	for _, v := range y {
		_, ok := hash[v]
		if ok {
			set = append(set, v)
		}
	}

	return set
}

// Difference returns the difference between two collections.
func Difference(x interface{}, y interface{}) (interface{}, interface{}) {
	if !IsCollection(x) {
		panic("First parameter must be a collection")
	}
	if !IsCollection(y) {
		panic("Second parameter must be a collection")
	}

	xValue := reflect.ValueOf(x)
	xType := xValue.Type()

	yValue := reflect.ValueOf(y)
	yType := yValue.Type()

	if NotEqual(xType, yType) {
		panic("Parameters must have the same type")
	}

	leftType := reflect.SliceOf(xType.Elem())
	leftSlice := reflect.MakeSlice(leftType, 0, 0)
	rightType := reflect.SliceOf(yType.Elem())
	rightSlice := reflect.MakeSlice(rightType, 0, 0)

	for i := 0; i < xValue.Len(); i++ {
		v := xValue.Index(i).Interface()
		if Contains(y, v) == false {
			leftSlice = reflect.Append(leftSlice, xValue.Index(i))
		}
	}

	for i := 0; i < yValue.Len(); i++ {
		v := yValue.Index(i).Interface()
		if Contains(x, v) == false {
			rightSlice = reflect.Append(rightSlice, yValue.Index(i))
		}
	}

	return leftSlice.Interface(), rightSlice.Interface()
}

// DifferenceString returns the difference between two collections of strings.
func DifferenceString(x []string, y []string) ([]string, []string) {
	leftSlice := []string{}
	rightSlice := []string{}

	for _, v := range x {
		if ContainsString(y, v) == false {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if ContainsString(x, v) == false {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}

// DifferenceInt64 returns the difference between two collections of int64s.
func DifferenceInt64(x []int64, y []int64) ([]int64, []int64) {
	leftSlice := []int64{}
	rightSlice := []int64{}

	for _, v := range x {
		if ContainsInt64(y, v) == false {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if ContainsInt64(x, v) == false {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}

// DifferenceInt32 returns the difference between two collections of ints32.
func DifferenceInt32(x []int32, y []int32) ([]int32, []int32) {
	leftSlice := []int32{}
	rightSlice := []int32{}

	for _, v := range x {
		if !ContainsInt32(y, v) {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if !ContainsInt32(x, v) {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}

// DifferenceInt returns the difference between two collections of ints.
func DifferenceInt(x []int, y []int) ([]int, []int) {
	leftSlice := []int{}
	rightSlice := []int{}

	for _, v := range x {
		if ContainsInt(y, v) == false {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if ContainsInt(x, v) == false {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}

// DifferenceUInt returns the difference between two collections of uints.
func DifferenceUInt(x []uint, y []uint) ([]uint, []uint) {
	leftSlice := []uint{}
	rightSlice := []uint{}

	for _, v := range x {
		if !ContainsUInt(y, v) {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if !ContainsUInt(x, v) {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}

// DifferenceUInt32 returns the difference between two collections of uints32.
func DifferenceUInt32(x []uint32, y []uint32) ([]uint32, []uint32) {
	leftSlice := []uint32{}
	rightSlice := []uint32{}

	for _, v := range x {
		if !ContainsUInt32(y, v) {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if !ContainsUInt32(x, v) {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}

// DifferenceUInt64 returns the difference between two collections of uints64.
func DifferenceUInt64(x []uint64, y []uint64) ([]uint64, []uint64) {
	leftSlice := []uint64{}
	rightSlice := []uint64{}

	for _, v := range x {
		if !ContainsUInt64(y, v) {
			leftSlice = append(leftSlice, v)
		}
	}

	for _, v := range y {
		if !ContainsUInt64(x, v) {
			rightSlice = append(rightSlice, v)
		}
	}

	return leftSlice, rightSlice
}
