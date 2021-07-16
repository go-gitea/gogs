package funk

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Set assigns in at path with value val. i.e. in.path = val
// in accepts types of ptr to struct, ptr to variable, slice and ptr to slice.
// Along the path, interface{} is supported and nil ptr is initialized to ptr to zero value
// of the type until the variable to be set is obtained.
// It returns errors when encountering along the path unknown types, uninitialized
// interface{} or interface{} containing struct directly (not ptr to struct).
//
// Slice is resolved the same way in funk.Get(), by traversing each element of the slice,
// so that each element of the slice's corresponding field are going to be set to the same provided val.
// If Set is called on slice with empty path "", it behaves the same as funk.Fill()
//
// If in is well formed, i.e. do not expect above descripted errors to happen, funk.MustSet()
// is a short hand wrapper to discard error return
func Set(in interface{}, val interface{}, path string) error {
	if in == nil {
		return errors.New("Cannot Set nil")
	}
	parts := []string{}
	if path != "" {
		parts = strings.Split(path, ".")
	}
	return setByParts(in, val, parts)
}

// we need this layer to handle interface{} type
func setByParts(in interface{}, val interface{}, parts []string) error {

	if in == nil {
		// nil interface can happen during traversing the path
		return errors.New("Cannot traverse nil/uninitialized interface{}")
	}

	inValue := reflect.ValueOf(in)
	inKind := inValue.Type().Kind()

	// Note: if interface contains a struct (not ptr to struct) then the content of the struct cannot be set.
	// I.e. it is not CanAddr() or CanSet()
	// So we require in interface{} to be a ptr, slice or array
	if inKind == reflect.Ptr {
		inValue = inValue.Elem() // if it is ptr we set its content not ptr its self
	} else if inKind != reflect.Array && inKind != reflect.Slice {
		return fmt.Errorf("Type %s not supported by Set", inValue.Type().String())
	}

	return set(inValue, reflect.ValueOf(val), parts)
}

// traverse inValue using path in parts and set the dst to be setValue
func set(inValue reflect.Value, setValue reflect.Value, parts []string) error {

	// traverse the path to get the inValue we need to set
	i := 0
	for i < len(parts) {

		kind := inValue.Kind()

		switch kind {
		case reflect.Invalid:
			// do not expect this case to happen
			return errors.New("nil pointer found along the path")
		case reflect.Struct:
			fValue := inValue.FieldByName(parts[i])
			if !fValue.IsValid() {
				return fmt.Errorf("field name %v is not found in struct %v", parts[i], inValue.Type().String())
			}
			if !fValue.CanSet() {
				return fmt.Errorf("field name %v is not exported in struct %v", parts[i], inValue.Type().String())
			}
			inValue = fValue
			i++
		case reflect.Slice | reflect.Array:
			// set all its elements
			length := inValue.Len()
			for j := 0; j < length; j++ {
				err := set(inValue.Index(j), setValue, parts[i:])
				if err != nil {
					return err
				}
			}
			return nil
		case reflect.Ptr:
			// only traverse down one level
			if inValue.IsNil() {
				// we initialize nil ptr to ptr to zero value of the type
				// and continue traversing
				inValue.Set(reflect.New(inValue.Type().Elem()))
			}
			// traverse the ptr until it is not pointer any more or is nil again
			inValue = redirectValue(inValue)
		case reflect.Interface:
			// Note: if interface contains a struct (not ptr to struct) then the content of the struct cannot be set.
			// I.e. it is not CanAddr() or CanSet(). This is why setByParts has a nil ptr check.
			// we treat this as a new call to setByParts, and it will do proper check of the types
			return setByParts(inValue.Interface(), setValue.Interface(), parts[i:])
		default:
			return fmt.Errorf("kind %v in path %v is not supported", kind, parts[i])
		}

	}
	// here inValue holds the value we need to set

	// interface{} can be set to any val
	// other types we ensure the type matches
	if inValue.Kind() != setValue.Kind() && inValue.Kind() != reflect.Interface {
		return fmt.Errorf("cannot set target of type %v with type %v", inValue.Kind(), setValue.Kind())
	}
	inValue.Set(setValue)

	return nil
}

// MustSet is functionally the same as Set.
// It panics instead of returning error.
// It is safe to use if the in value is well formed.
func MustSet(in interface{}, val interface{}, path string) {
	err := Set(in, val, path)
	if err != nil {
		panic(err)
	}
}
