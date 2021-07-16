package funk

import (
	"fmt"
	"reflect"
)

// ForEach iterates over elements of collection and invokes iteratee
// for each element.
func ForEach(arr interface{}, predicate interface{}) {
	if !IsIteratee(arr) {
		panic("First parameter must be an iteratee")
	}

	var (
		funcValue = reflect.ValueOf(predicate)
		arrValue  = reflect.ValueOf(arr)
		arrType   = arrValue.Type()
		funcType  = funcValue.Type()
	)

	if arrType.Kind() == reflect.Slice || arrType.Kind() == reflect.Array {
		if !IsFunction(predicate, 1, 0) {
			panic("Second argument must be a function with one parameter")
		}

		arrElemType := arrValue.Type().Elem()

		// Checking whether element type is convertible to function's first argument's type.
		if !arrElemType.ConvertibleTo(funcType.In(0)) {
			panic("Map function's argument is not compatible with type of array.")
		}

		for i := 0; i < arrValue.Len(); i++ {
			funcValue.Call([]reflect.Value{arrValue.Index(i)})
		}
	}

	if arrType.Kind() == reflect.Map {
		if !IsFunction(predicate, 2, 0) {
			panic("Second argument must be a function with two parameters")
		}

		// Type checking for Map<key, value> = (key, value)
		keyType := arrType.Key()
		valueType := arrType.Elem()

		if !keyType.ConvertibleTo(funcType.In(0)) {
			panic(fmt.Sprintf("function first argument is not compatible with %s", keyType.String()))
		}

		if !valueType.ConvertibleTo(funcType.In(1)) {
			panic(fmt.Sprintf("function second argument is not compatible with %s", valueType.String()))
		}

		for _, key := range arrValue.MapKeys() {
			funcValue.Call([]reflect.Value{key, arrValue.MapIndex(key)})
		}
	}
}

// ForEachRight iterates over elements of collection from the right and invokes iteratee
// for each element.
func ForEachRight(arr interface{}, predicate interface{}) {
	if !IsIteratee(arr) {
		panic("First parameter must be an iteratee")
	}

	var (
		funcValue = reflect.ValueOf(predicate)
		arrValue  = reflect.ValueOf(arr)
		arrType   = arrValue.Type()
		funcType  = funcValue.Type()
	)

	if arrType.Kind() == reflect.Slice || arrType.Kind() == reflect.Array {
		if !IsFunction(predicate, 1, 0) {
			panic("Second argument must be a function with one parameter")
		}

		arrElemType := arrValue.Type().Elem()

		// Checking whether element type is convertible to function's first argument's type.
		if !arrElemType.ConvertibleTo(funcType.In(0)) {
			panic("Map function's argument is not compatible with type of array.")
		}

		for i := arrValue.Len() - 1; i >= 0; i-- {
			funcValue.Call([]reflect.Value{arrValue.Index(i)})
		}
	}

	if arrType.Kind() == reflect.Map {
		if !IsFunction(predicate, 2, 0) {
			panic("Second argument must be a function with two parameters")
		}

		// Type checking for Map<key, value> = (key, value)
		keyType := arrType.Key()
		valueType := arrType.Elem()

		if !keyType.ConvertibleTo(funcType.In(0)) {
			panic(fmt.Sprintf("function first argument is not compatible with %s", keyType.String()))
		}

		if !valueType.ConvertibleTo(funcType.In(1)) {
			panic(fmt.Sprintf("function second argument is not compatible with %s", valueType.String()))
		}

		keys := Reverse(arrValue.MapKeys()).([]reflect.Value)

		for _, key := range keys {
			funcValue.Call([]reflect.Value{key, arrValue.MapIndex(key)})
		}
	}
}

// Head gets the first element of array.
func Head(arr interface{}) interface{} {
	value := redirectValue(reflect.ValueOf(arr))
	valueType := value.Type()

	kind := value.Kind()

	if kind == reflect.Array || kind == reflect.Slice {
		if value.Len() == 0 {
			return nil
		}

		return value.Index(0).Interface()
	}

	panic(fmt.Sprintf("Type %s is not supported by Head", valueType.String()))
}

// Last gets the last element of array.
func Last(arr interface{}) interface{} {
	value := redirectValue(reflect.ValueOf(arr))
	valueType := value.Type()

	kind := value.Kind()

	if kind == reflect.Array || kind == reflect.Slice {
		if value.Len() == 0 {
			return nil
		}

		return value.Index(value.Len() - 1).Interface()
	}

	panic(fmt.Sprintf("Type %s is not supported by Last", valueType.String()))
}

// Initial gets all but the last element of array.
func Initial(arr interface{}) interface{} {
	value := redirectValue(reflect.ValueOf(arr))
	valueType := value.Type()

	kind := value.Kind()

	if kind == reflect.Array || kind == reflect.Slice {
		length := value.Len()

		if length <= 1 {
			return arr
		}

		return value.Slice(0, length-1).Interface()
	}

	panic(fmt.Sprintf("Type %s is not supported by Initial", valueType.String()))
}

// Tail gets all but the first element of array.
func Tail(arr interface{}) interface{} {
	value := redirectValue(reflect.ValueOf(arr))
	valueType := value.Type()

	kind := value.Kind()

	if kind == reflect.Array || kind == reflect.Slice {
		length := value.Len()

		if length <= 1 {
			return arr
		}

		return value.Slice(1, length).Interface()
	}

	panic(fmt.Sprintf("Type %s is not supported by Initial", valueType.String()))
}
