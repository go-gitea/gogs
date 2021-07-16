package funk

import (
	"fmt"
	"reflect"
)

// Keys creates an array of the own enumerable map keys or struct field names.
func Keys(out interface{}) interface{} {
	value := redirectValue(reflect.ValueOf(out))
	valueType := value.Type()

	if value.Kind() == reflect.Map {
		keys := value.MapKeys()

		length := len(keys)

		resultSlice := reflect.MakeSlice(reflect.SliceOf(valueType.Key()), length, length)

		for i, key := range keys {
			resultSlice.Index(i).Set(key)
		}

		return resultSlice.Interface()
	}

	if value.Kind() == reflect.Struct {
		length := value.NumField()

		resultSlice := make([]string, length)

		for i := 0; i < length; i++ {
			resultSlice[i] = valueType.Field(i).Name
		}

		return resultSlice
	}

	panic(fmt.Sprintf("Type %s is not supported by Keys", valueType.String()))
}

// Values creates an array of the own enumerable map values or struct field values.
func Values(out interface{}) interface{} {
	value := redirectValue(reflect.ValueOf(out))
	valueType := value.Type()

	if value.Kind() == reflect.Map {
		keys := value.MapKeys()

		length := len(keys)

		resultSlice := reflect.MakeSlice(reflect.SliceOf(valueType.Elem()), length, length)

		for i, key := range keys {
			resultSlice.Index(i).Set(value.MapIndex(key))
		}

		return resultSlice.Interface()
	}

	if value.Kind() == reflect.Struct {
		length := value.NumField()

		resultSlice := make([]interface{}, length)

		for i := 0; i < length; i++ {
			resultSlice[i] = value.Field(i).Interface()
		}

		return resultSlice
	}

	panic(fmt.Sprintf("Type %s is not supported by Keys", valueType.String()))
}
