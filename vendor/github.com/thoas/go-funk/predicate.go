package funk

import (
	"reflect"
)

// predicatesImpl contains the common implementation of AnyPredicates and AllPredicates.
func predicatesImpl(value interface{}, wantedAnswer bool, predicates interface{}) bool {
	if !IsCollection(predicates) {
		panic("Predicates parameter must be an iteratee")
	}

	predicatesValue := reflect.ValueOf(predicates)
	inputValue := reflect.ValueOf(value)

	for i := 0; i < predicatesValue.Len(); i++ {
		funcValue := predicatesValue.Index(i)
		if !IsFunction(funcValue.Interface()) {
			panic("Got non function as predicate")
		}

		funcType := funcValue.Type()
		if !IsPredicate(funcValue.Interface()) {
			panic("Predicate function must have 1 parameter and must return boolean")
		}

		if !inputValue.Type().ConvertibleTo(funcType.In(0)) {
			panic("Given value is not compatible with type of parameter for the predicate.")
		}
		if result := funcValue.Call([]reflect.Value{inputValue}); wantedAnswer == result[0].Bool() {
			return wantedAnswer
		}
	}

	return !wantedAnswer
}

// AnyPredicates gets a value and a series of predicates, and return true if at least one of the predicates
// is true.
func AnyPredicates(value interface{}, predicates interface{}) bool {
	return predicatesImpl(value, true, predicates)
}

// AllPredicates gets a value and a series of predicates, and return true if all of the predicates are true.
func AllPredicates(value interface{}, predicates interface{}) bool {
	return predicatesImpl(value, false, predicates)
}
