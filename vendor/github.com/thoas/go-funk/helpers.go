package funk

import (
	"bytes"
	"math/rand"
	"reflect"
)

var numericZeros = []interface{}{
	int(0),
	int8(0),
	int16(0),
	int32(0),
	int64(0),
	uint(0),
	uint8(0),
	uint16(0),
	uint32(0),
	uint64(0),
	float32(0),
	float64(0),
}

// ToFloat64 converts any numeric value to float64.
func ToFloat64(x interface{}) (float64, bool) {
	var xf float64
	xok := true

	switch xn := x.(type) {
	case uint8:
		xf = float64(xn)
	case uint16:
		xf = float64(xn)
	case uint32:
		xf = float64(xn)
	case uint64:
		xf = float64(xn)
	case int:
		xf = float64(xn)
	case int8:
		xf = float64(xn)
	case int16:
		xf = float64(xn)
	case int32:
		xf = float64(xn)
	case int64:
		xf = float64(xn)
	case float32:
		xf = float64(xn)
	case float64:
		xf = float64(xn)
	default:
		xok = false
	}

	return xf, xok
}

// PtrOf makes a copy of the given interface and returns a pointer.
func PtrOf(itf interface{}) interface{} {
	t := reflect.TypeOf(itf)

	cp := reflect.New(t)
	cp.Elem().Set(reflect.ValueOf(itf))

	// Avoid double pointers if itf is a pointer
	if t.Kind() == reflect.Ptr {
		return cp.Elem().Interface()
	}

	return cp.Interface()
}

// IsFunction returns if the argument is a function.
func IsFunction(in interface{}, num ...int) bool {
	funcType := reflect.TypeOf(in)

	result := funcType != nil && funcType.Kind() == reflect.Func

	if len(num) >= 1 {
		result = result && funcType.NumIn() == num[0]
	}

	if len(num) == 2 {
		result = result && funcType.NumOut() == num[1]
	}

	return result
}

// IsPredicate returns if the argument is a predicate function.
func IsPredicate(in interface{}, inTypes ...reflect.Type) bool {
	if len(inTypes) == 0 {
		inTypes = append(inTypes, nil)
	}

	funcType := reflect.TypeOf(in)

	result := funcType != nil && funcType.Kind() == reflect.Func

	result = result && funcType.NumOut() == 1 && funcType.Out(0).Kind() == reflect.Bool
	result = result && funcType.NumIn() == len(inTypes)

	for i := 0; result && i < len(inTypes); i++ {
		inType := inTypes[i]
		result = inType == nil || inType.ConvertibleTo(funcType.In(i))
	}

	return result
}

// IsEqual returns if the two objects are equal
func IsEqual(expected interface{}, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	if exp, ok := expected.([]byte); ok {
		act, ok := actual.([]byte)
		if !ok {
			return false
		}

		if exp == nil || act == nil {
			return true
		}

		return bytes.Equal(exp, act)
	}

	return reflect.DeepEqual(expected, actual)

}

// IsType returns if the two objects are in the same type
func IsType(expected interface{}, actual interface{}) bool {
	return IsEqual(reflect.TypeOf(expected), reflect.TypeOf(actual))
}

// Equal returns if the two objects are equal
func Equal(expected interface{}, actual interface{}) bool {
	return IsEqual(expected, actual)
}

// NotEqual returns if the two objects are not equal
func NotEqual(expected interface{}, actual interface{}) bool {
	return !IsEqual(expected, actual)
}

// IsIteratee returns if the argument is an iteratee.
func IsIteratee(in interface{}) bool {
	if in == nil {
		return false
	}
	arrType := reflect.TypeOf(in)

	kind := arrType.Kind()

	return kind == reflect.Array || kind == reflect.Slice || kind == reflect.Map
}

// IsCollection returns if the argument is a collection.
func IsCollection(in interface{}) bool {
	arrType := reflect.TypeOf(in)

	kind := arrType.Kind()

	return kind == reflect.Array || kind == reflect.Slice
}

// SliceOf returns a slice which contains the element.
func SliceOf(in interface{}) interface{} {
	value := reflect.ValueOf(in)

	sliceType := reflect.SliceOf(reflect.TypeOf(in))
	slice := reflect.New(sliceType)
	sliceValue := reflect.MakeSlice(sliceType, 0, 0)
	sliceValue = reflect.Append(sliceValue, value)
	slice.Elem().Set(sliceValue)

	return slice.Elem().Interface()
}

// Any returns true if any element of the iterable is not empty. If the iterable is empty, return False.
func Any(objs ...interface{}) bool {
	if len(objs) == 0 {
		return false
	}

	for _, obj := range objs {
		if !IsEmpty(obj) {
			return true
		}
	}

	return false
}

// All returns true if all elements of the iterable are not empty (or if the iterable is empty)
func All(objs ...interface{}) bool {
	if len(objs) == 0 {
		return true
	}

	for _, obj := range objs {
		if IsEmpty(obj) {
			return false
		}
	}

	return true
}

// IsEmpty returns if the object is considered as empty or not.
func IsEmpty(obj interface{}) bool {
	if obj == nil || obj == "" || obj == false {
		return true
	}

	for _, v := range numericZeros {
		if obj == v {
			return true
		}
	}

	objValue := reflect.ValueOf(obj)

	switch objValue.Kind() {
	case reflect.Map:
		fallthrough
	case reflect.Slice, reflect.Chan:
		return objValue.Len() == 0
	case reflect.Struct:
		return reflect.DeepEqual(obj, ZeroOf(obj))
	case reflect.Ptr:
		if objValue.IsNil() {
			return true
		}

		obj = redirectValue(objValue).Interface()

		return reflect.DeepEqual(obj, ZeroOf(obj))
	}

	return false
}

// IsZero returns if the object is considered as zero value
func IsZero(obj interface{}) bool {
	if obj == nil || obj == "" || obj == false {
		return true
	}

	for _, v := range numericZeros {
		if obj == v {
			return true
		}
	}

	return reflect.DeepEqual(obj, ZeroOf(obj))
}

// NotEmpty returns if the object is considered as non-empty or not.
func NotEmpty(obj interface{}) bool {
	return !IsEmpty(obj)
}

// ZeroOf returns a zero value of an element.
func ZeroOf(in interface{}) interface{} {
	if in == nil {
		return nil
	}

	return reflect.Zero(reflect.TypeOf(in)).Interface()
}

// RandomInt generates a random int, based on a min and max values
func RandomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Shard will shard a string name
func Shard(str string, width int, depth int, restOnly bool) []string {
	var results []string

	for i := 0; i < depth; i++ {
		results = append(results, str[(width*i):(width*(i+1))])
	}

	if restOnly {
		results = append(results, str[(width*depth):])
	} else {
		results = append(results, str)
	}

	return results
}

var defaultLetters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// RandomString returns a random string with a fixed length
func RandomString(n int, allowedChars ...[]rune) string {
	var letters []rune

	if len(allowedChars) == 0 {
		letters = defaultLetters
	} else {
		letters = allowedChars[0]
	}

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}
