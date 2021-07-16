package funk

import "reflect"

// Without creates an array excluding all given values.
func Without(in interface{}, values ...interface{}) interface{} {
	if !IsCollection(in) {
		panic("First parameter must be a collection")
	}

	inValue := reflect.ValueOf(in)
	for _, value := range values {
		if NotEqual(inValue.Type().Elem(), reflect.TypeOf(value)) {
			panic("Values must have the same type")
		}
	}

	return LeftJoin(inValue, reflect.ValueOf(values)).Interface()
}
