package funk

import (
	"reflect"
)

// Subset returns true if collection x is a subset of y.
func Subset(x interface{}, y interface{}) bool {
	if !IsCollection(x) {
		panic("First parameter must be a collection")
	}
	if !IsCollection(y) {
		panic("Second parameter must be a collection")
	}

	xValue := reflect.ValueOf(x)
	xType := xValue.Type()

	yValue := reflect.ValueOf(y)
	yType := yValue.Type()

	if NotEqual(xType, yType) {
		panic("Parameters must have the same type")
	}

	if xValue.Len() == 0 {
		return true
	}

	if yValue.Len() == 0 || yValue.Len() < xValue.Len() {
		return false
	}

	for i := 0; i < xValue.Len(); i++ {
		if !Contains(yValue.Interface(), xValue.Index(i).Interface()) {
			return false
		}
	}

    return true
}
