package funk

import (
	"reflect"
)

// Compact creates a slice with all empty/zero values removed.
func Compact(value interface{}) interface{} {
	arr := redirectValue(reflect.ValueOf(value))

	if arr.Kind() != reflect.Slice && arr.Kind() != reflect.Array {
		panic("First parameter must be array or slice")
	}

	sliceElemType := sliceElem(arr.Type())
	resultSliceType := reflect.SliceOf(sliceElemType)
	result := reflect.MakeSlice(resultSliceType, 0, 0)

	for i := 0; i < arr.Len(); i++ {
		elemVal := arr.Index(i)

		if elemVal.Kind() == reflect.Interface {
			elemVal = elemVal.Elem()
		}

		redirectedElemVal := redirectValue(elemVal)

		switch redirectedElemVal.Kind() {
		case reflect.Invalid:
			continue
		case reflect.Func:
			if redirectedElemVal.IsNil() {
				continue
			}
		case reflect.Map, reflect.Slice, reflect.Chan:
			if redirectedElemVal.Len() == 0 {
				continue
			}
		default:
			defaultValue := reflect.Zero(redirectedElemVal.Type()).Interface()
			if redirectedElemVal.Interface() == defaultValue {
				continue
			}
		}

		result = reflect.Append(result, elemVal)
	}

	return result.Interface()
}
