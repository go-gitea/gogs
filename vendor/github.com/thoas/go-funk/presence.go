package funk

import (
	"fmt"
	"reflect"
	"strings"
)

// Filter iterates over elements of collection, returning an array of
// all elements predicate returns truthy for.
func Filter(arr interface{}, predicate interface{}) interface{} {
	if !IsIteratee(arr) {
		panic("First parameter must be an iteratee")
	}

	if !IsFunction(predicate, 1, 1) {
		panic("Second argument must be function")
	}

	funcValue := reflect.ValueOf(predicate)

	funcType := funcValue.Type()

	if funcType.Out(0).Kind() != reflect.Bool {
		panic("Return argument should be a boolean")
	}

	arrValue := reflect.ValueOf(arr)

	arrType := arrValue.Type()

	// Get slice type corresponding to array type
	resultSliceType := reflect.SliceOf(arrType.Elem())

	// MakeSlice takes a slice kind type, and makes a slice.
	resultSlice := reflect.MakeSlice(resultSliceType, 0, 0)

	for i := 0; i < arrValue.Len(); i++ {
		elem := arrValue.Index(i)

		result := funcValue.Call([]reflect.Value{elem})[0].Interface().(bool)

		if result {
			resultSlice = reflect.Append(resultSlice, elem)
		}
	}

	return resultSlice.Interface()
}

// Find iterates over elements of collection, returning the first
// element predicate returns truthy for.
func Find(arr interface{}, predicate interface{}) interface{} {
	_, val := FindKey(arr, predicate)
	return val
}

// Find iterates over elements of collection, returning the first
// element of an array and random of a map which predicate returns truthy for.
func FindKey(arr interface{}, predicate interface{}) (matchKey, matchEle interface{}) {
	if !IsIteratee(arr) {
		panic("First parameter must be an iteratee")
	}

	if !IsFunction(predicate, 1, 1) {
		panic("Second argument must be function")
	}

	funcValue := reflect.ValueOf(predicate)

	funcType := funcValue.Type()

	if funcType.Out(0).Kind() != reflect.Bool {
		panic("Return argument should be a boolean")
	}

	arrValue := reflect.ValueOf(arr)
	var keyArrs []reflect.Value

	isMap := arrValue.Kind() == reflect.Map
	if isMap {
		keyArrs = arrValue.MapKeys()
	}
	for i := 0; i < arrValue.Len(); i++ {
		var (
			elem reflect.Value
			key  reflect.Value
		)
		if isMap {
			key = keyArrs[i]
			elem = arrValue.MapIndex(key)
		} else {
			key = reflect.ValueOf(i)
			elem = arrValue.Index(i)
		}

		result := funcValue.Call([]reflect.Value{elem})[0].Interface().(bool)

		if result {
			return key.Interface(), elem.Interface()
		}
	}

	return nil, nil
}

// IndexOf gets the index at which the first occurrence of value is found in array or return -1
// if the value cannot be found
func IndexOf(in interface{}, elem interface{}) int {
	inValue := reflect.ValueOf(in)

	elemValue := reflect.ValueOf(elem)

	inType := inValue.Type()

	if inType.Kind() == reflect.String {
		return strings.Index(inValue.String(), elemValue.String())
	}

	if inType.Kind() == reflect.Slice {
		equalTo := equal(elem)
		for i := 0; i < inValue.Len(); i++ {
			if equalTo(reflect.Value{}, inValue.Index(i)) {
				return i
			}
		}
	}

	return -1
}

// LastIndexOf gets the index at which the last occurrence of value is found in array or return -1
// if the value cannot be found
func LastIndexOf(in interface{}, elem interface{}) int {
	inValue := reflect.ValueOf(in)

	elemValue := reflect.ValueOf(elem)

	inType := inValue.Type()

	if inType.Kind() == reflect.String {
		return strings.LastIndex(inValue.String(), elemValue.String())
	}

	if inType.Kind() == reflect.Slice {
		length := inValue.Len()

		equalTo := equal(elem)
		for i := length - 1; i >= 0; i-- {
			if equalTo(reflect.Value{}, inValue.Index(i)) {
				return i
			}
		}
	}

	return -1
}

// Contains returns true if an element is present in a iteratee.
func Contains(in interface{}, elem interface{}) bool {
	inValue := reflect.ValueOf(in)
	elemValue := reflect.ValueOf(elem)
	inType := inValue.Type()

	switch inType.Kind() {
	case reflect.String:
		return strings.Contains(inValue.String(), elemValue.String())
	case reflect.Map:
		equalTo := equal(elem, true)
		for _, key := range inValue.MapKeys() {
			if equalTo(key, inValue.MapIndex(key)) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		equalTo := equal(elem)
		for i := 0; i < inValue.Len(); i++ {
			if equalTo(reflect.Value{}, inValue.Index(i)) {
				return true
			}
		}
	default:
		panic(fmt.Sprintf("Type %s is not supported by Contains, supported types are String, Map, Slice, Array", inType.String()))
	}

	return false
}

// Every returns true if every element is present in a iteratee.
func Every(in interface{}, elements ...interface{}) bool {
	for _, elem := range elements {
		if !Contains(in, elem) {
			return false
		}
	}
	return true
}

// Some returns true if atleast one element is present in an iteratee.
func Some(in interface{}, elements ...interface{}) bool {
	for _, elem := range elements {
		if Contains(in, elem) {
			return true
		}
	}
	return false
}
