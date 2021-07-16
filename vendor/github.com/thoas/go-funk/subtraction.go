package funk

import (
	"reflect"
)

// Subtract returns the subtraction between two collections.
func Subtract(x interface{}, y interface{}) interface{} {
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
			delete(hash, v)
		}
	}

	for i := 0; i < xValue.Len(); i++ {
		v := xValue.Index(i).Interface()
		_, ok := hash[v]
		if ok {
			zSlice = reflect.Append(zSlice, xValue.Index(i))
		}
	}

	return zSlice.Interface()
}

// SubtractString returns the subtraction between two collections of string
func SubtractString(x []string, y []string) []string {
	if len(x) == 0 {
		return []string{}
	}

	if len(y) == 0 {
		return x
	}

	slice := []string{}
	hash := map[string]struct{}{}

	for _, v := range x {
		hash[v] = struct{}{}
	}

	for _, v := range y {
		_, ok := hash[v]
		if ok {
			delete(hash, v)
		}
	}

	for _, v := range x {
		_, ok := hash[v]
		if ok {
			slice = append(slice, v)
		}
	}

	return slice
}
