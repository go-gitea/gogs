package funk

import (
	"reflect"
)

// Tuple is the return type of Zip
type Tuple struct {
	Element1 interface{}
	Element2 interface{}
}

// Zip returns a list of tuples, where the i-th tuple contains the i-th element
// from each of the input iterables. The returned list is truncated in length
// to the length of the shortest input iterable.
func Zip(slice1 interface{}, slice2 interface{}) []Tuple {
	inValue1 := reflect.ValueOf(slice1)
	inValue2 := reflect.ValueOf(slice2)
	kind1 := inValue1.Type().Kind()
	kind2 := inValue2.Type().Kind()

	result := []Tuple{}
	for _, kind := range []reflect.Kind{kind1, kind2} {
		if kind != reflect.Slice && kind != reflect.Array {
			return result
		}
	}

	var minLength int
	length1 := inValue1.Len()
	length2 := inValue2.Len()
	if length1 <= length2 {
		minLength = length1
	} else {
		minLength = length2
	}

	for i := 0; i < minLength; i++ {
		newTuple := Tuple{
			Element1: inValue1.Index(i).Interface(),
			Element2: inValue2.Index(i).Interface(),
		}
		result = append(result, newTuple)
	}
	return result
}
