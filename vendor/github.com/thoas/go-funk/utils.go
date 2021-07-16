package funk

import (
	"fmt"
	"reflect"
)

func equal(expectedOrPredicate interface{}, optionalIsMap ...bool) func(keyValueIfMap, actualValue reflect.Value) bool {
	isMap := append(optionalIsMap, false)[0]

	if IsFunction(expectedOrPredicate) {
		inTypes := []reflect.Type{nil}; if isMap {
			inTypes = append(inTypes, nil)
		}

		if !IsPredicate(expectedOrPredicate, inTypes...) {
			panic(fmt.Sprintf("Predicate function must have %d parameter and must return boolean", len(inTypes)))
		}

		predicateValue := reflect.ValueOf(expectedOrPredicate)

		return func(keyValueIfMap, actualValue reflect.Value) bool {

			if isMap && !keyValueIfMap.Type().ConvertibleTo(predicateValue.Type().In(0)) {
				panic("Given key is not compatible with type of parameter for the predicate.")
			}

			if (isMap && !actualValue.Type().ConvertibleTo(predicateValue.Type().In(1))) ||
				(!isMap && !actualValue.Type().ConvertibleTo(predicateValue.Type().In(0))) {
				panic("Given value is not compatible with type of parameter for the predicate.")
			}

			args := []reflect.Value{actualValue}
			if isMap {
				args = append([]reflect.Value{keyValueIfMap}, args...)
			}

			return predicateValue.Call(args)[0].Bool()
		}
	}

	expected := expectedOrPredicate

	return func(keyValueIfMap, actualValue reflect.Value) bool {
		if isMap {
			actualValue = keyValueIfMap
		}

		if expected == nil || actualValue.IsZero() {
			return actualValue.Interface() == expected
		}

		return reflect.DeepEqual(actualValue.Interface(), expected)
	}
}

func sliceElem(rtype reflect.Type) reflect.Type {
	for {
		if rtype.Kind() != reflect.Slice && rtype.Kind() != reflect.Array {
			return rtype
		}

		rtype = rtype.Elem()
	}
}

func redirectValue(value reflect.Value) reflect.Value {
	for {
		if !value.IsValid() || (value.Kind() != reflect.Ptr && value.Kind() != reflect.Interface) {
			return value
		}

		res := value.Elem()

		// Test for a circular type.
		if res.Kind() == reflect.Ptr && value.Kind() == reflect.Ptr && value.Pointer() == res.Pointer() {
			return value
		}

		if !res.IsValid() && value.Kind() == reflect.Ptr {
			return reflect.Zero(value.Type().Elem())
		}

		value = res
	}
}

func makeSlice(value reflect.Value, values ...int) reflect.Value {
	sliceType := sliceElem(value.Type())

	size := value.Len()
	cap := size

	if len(values) > 0 {
		size = values[0]
	}

	if len(values) > 1 {
		cap = values[1]
	}

	return reflect.MakeSlice(reflect.SliceOf(sliceType), size, cap)
}
