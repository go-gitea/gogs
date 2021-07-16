package funk

import (
	"reflect"
	"strings"
)

// Get retrieves the value at path of struct(s).
func Get(out interface{}, path string) interface{} {
	result := get(reflect.ValueOf(out), path)

	if result.Kind() != reflect.Invalid && !result.IsZero() {
		return result.Interface()
	}

	return nil
}

func get(value reflect.Value, path string) reflect.Value {
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		var resultSlice reflect.Value

		length := value.Len()

		if length == 0 {
			zeroElement := reflect.Zero(value.Type().Elem())
			pathValue := get(zeroElement, path)
			value = reflect.MakeSlice(reflect.SliceOf(pathValue.Type()), 0, 0)

			return value
		}

		for i := 0; i < length; i++ {
			item := value.Index(i)

			resultValue := get(item, path)

			if resultValue.Kind() == reflect.Invalid || resultValue.IsZero() {
				continue
			}

			resultType := resultValue.Type()

			if resultSlice.Kind() == reflect.Invalid {
				resultType := reflect.SliceOf(resultType)

				resultSlice = reflect.MakeSlice(resultType, 0, 0)
			}

			resultSlice = reflect.Append(resultSlice, resultValue)
		}

		// if the result is a slice of a slice, we need to flatten it
		if resultSlice.Kind() != reflect.Invalid && resultSlice.Type().Elem().Kind() == reflect.Slice {
			return flattenDeep(resultSlice)
		}

		return resultSlice
	}

	parts := strings.Split(path, ".")

	for _, part := range parts {
		value = redirectValue(value)
		kind := value.Kind()

		switch kind {
		case reflect.Invalid:
			continue
		case reflect.Struct:
			value = value.FieldByName(part)
		case reflect.Map:
			value = value.MapIndex(reflect.ValueOf(part))
		case reflect.Slice, reflect.Array:
			value = get(value, part)
		default:
			return reflect.ValueOf(nil)
		}
	}

	return value
}

// Get retrieves the value of the pointer or default.
func GetOrElse(v interface{}, def interface{}) interface{} {
	val := reflect.ValueOf(v)
	if v == nil || (val.Kind() == reflect.Ptr && val.IsNil()) {
		return def
	} else if val.Kind() != reflect.Ptr {
		return v
	}
	return val.Elem().Interface()
}
