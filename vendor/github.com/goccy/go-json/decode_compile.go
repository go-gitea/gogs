package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unsafe"

	"github.com/goccy/go-json/internal/runtime"
)

var (
	jsonNumberType = reflect.TypeOf(json.Number(""))
)

func decodeCompileToGetDecoderSlowPath(typeptr uintptr, typ *rtype) (decoder, error) {
	decoderMap := loadDecoderMap()
	if dec, exists := decoderMap[typeptr]; exists {
		return dec, nil
	}

	dec, err := decodeCompileHead(typ, map[uintptr]decoder{})
	if err != nil {
		return nil, err
	}
	storeDecoder(typeptr, dec, decoderMap)
	return dec, nil
}

func decodeCompileHead(typ *rtype, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	switch {
	case rtype_ptrTo(typ).Implements(unmarshalJSONType):
		return newUnmarshalJSONDecoder(rtype_ptrTo(typ), "", ""), nil
	case rtype_ptrTo(typ).Implements(unmarshalTextType):
		return newUnmarshalTextDecoder(rtype_ptrTo(typ), "", ""), nil
	}
	return decodeCompile(typ.Elem(), "", "", structTypeToDecoder)
}

func decodeCompile(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	switch {
	case rtype_ptrTo(typ).Implements(unmarshalJSONType):
		return newUnmarshalJSONDecoder(rtype_ptrTo(typ), structName, fieldName), nil
	case rtype_ptrTo(typ).Implements(unmarshalTextType):
		return newUnmarshalTextDecoder(rtype_ptrTo(typ), structName, fieldName), nil
	}

	switch typ.Kind() {
	case reflect.Ptr:
		return decodeCompilePtr(typ, structName, fieldName, structTypeToDecoder)
	case reflect.Struct:
		return decodeCompileStruct(typ, structName, fieldName, structTypeToDecoder)
	case reflect.Slice:
		elem := typ.Elem()
		if elem.Kind() == reflect.Uint8 {
			return decodeCompileBytes(elem, structName, fieldName)
		}
		return decodeCompileSlice(typ, structName, fieldName, structTypeToDecoder)
	case reflect.Array:
		return decodeCompileArray(typ, structName, fieldName, structTypeToDecoder)
	case reflect.Map:
		return decodeCompileMap(typ, structName, fieldName, structTypeToDecoder)
	case reflect.Interface:
		return decodeCompileInterface(typ, structName, fieldName)
	case reflect.Uintptr:
		return decodeCompileUint(typ, structName, fieldName)
	case reflect.Int:
		return decodeCompileInt(typ, structName, fieldName)
	case reflect.Int8:
		return decodeCompileInt8(typ, structName, fieldName)
	case reflect.Int16:
		return decodeCompileInt16(typ, structName, fieldName)
	case reflect.Int32:
		return decodeCompileInt32(typ, structName, fieldName)
	case reflect.Int64:
		return decodeCompileInt64(typ, structName, fieldName)
	case reflect.Uint:
		return decodeCompileUint(typ, structName, fieldName)
	case reflect.Uint8:
		return decodeCompileUint8(typ, structName, fieldName)
	case reflect.Uint16:
		return decodeCompileUint16(typ, structName, fieldName)
	case reflect.Uint32:
		return decodeCompileUint32(typ, structName, fieldName)
	case reflect.Uint64:
		return decodeCompileUint64(typ, structName, fieldName)
	case reflect.String:
		return decodeCompileString(typ, structName, fieldName)
	case reflect.Bool:
		return decodeCompileBool(structName, fieldName)
	case reflect.Float32:
		return decodeCompileFloat32(structName, fieldName)
	case reflect.Float64:
		return decodeCompileFloat64(structName, fieldName)
	}
	return nil, &UnmarshalTypeError{
		Value:  "object",
		Type:   rtype2type(typ),
		Offset: 0,
	}
}

func isStringTagSupportedType(typ *rtype) bool {
	switch {
	case rtype_ptrTo(typ).Implements(unmarshalJSONType):
		return false
	case rtype_ptrTo(typ).Implements(unmarshalTextType):
		return false
	}
	switch typ.Kind() {
	case reflect.Map:
		return false
	case reflect.Slice:
		return false
	case reflect.Array:
		return false
	case reflect.Struct:
		return false
	case reflect.Interface:
		return false
	}
	return true
}

func decodeCompileMapKey(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	if rtype_ptrTo(typ).Implements(unmarshalTextType) {
		return newUnmarshalTextDecoder(rtype_ptrTo(typ), structName, fieldName), nil
	}
	dec, err := decodeCompile(typ, structName, fieldName, structTypeToDecoder)
	if err != nil {
		return nil, err
	}
	for {
		switch t := dec.(type) {
		case *stringDecoder, *interfaceDecoder:
			return dec, nil
		case *boolDecoder, *intDecoder, *uintDecoder, *numberDecoder:
			return newWrappedStringDecoder(typ, dec, structName, fieldName), nil
		case *ptrDecoder:
			dec = t.dec
		default:
			goto ERROR
		}
	}
ERROR:
	return nil, &UnmarshalTypeError{
		Value:  "object",
		Type:   rtype2type(typ),
		Offset: 0,
	}
}

func decodeCompilePtr(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	dec, err := decodeCompile(typ.Elem(), structName, fieldName, structTypeToDecoder)
	if err != nil {
		return nil, err
	}
	return newPtrDecoder(dec, typ.Elem(), structName, fieldName), nil
}

func decodeCompileInt(typ *rtype, structName, fieldName string) (decoder, error) {
	return newIntDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v int64) {
		*(*int)(p) = int(v)
	}), nil
}

func decodeCompileInt8(typ *rtype, structName, fieldName string) (decoder, error) {
	return newIntDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v int64) {
		*(*int8)(p) = int8(v)
	}), nil
}

func decodeCompileInt16(typ *rtype, structName, fieldName string) (decoder, error) {
	return newIntDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v int64) {
		*(*int16)(p) = int16(v)
	}), nil
}

func decodeCompileInt32(typ *rtype, structName, fieldName string) (decoder, error) {
	return newIntDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v int64) {
		*(*int32)(p) = int32(v)
	}), nil
}

func decodeCompileInt64(typ *rtype, structName, fieldName string) (decoder, error) {
	return newIntDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v int64) {
		*(*int64)(p) = v
	}), nil
}

func decodeCompileUint(typ *rtype, structName, fieldName string) (decoder, error) {
	return newUintDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v uint64) {
		*(*uint)(p) = uint(v)
	}), nil
}

func decodeCompileUint8(typ *rtype, structName, fieldName string) (decoder, error) {
	return newUintDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v uint64) {
		*(*uint8)(p) = uint8(v)
	}), nil
}

func decodeCompileUint16(typ *rtype, structName, fieldName string) (decoder, error) {
	return newUintDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v uint64) {
		*(*uint16)(p) = uint16(v)
	}), nil
}

func decodeCompileUint32(typ *rtype, structName, fieldName string) (decoder, error) {
	return newUintDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v uint64) {
		*(*uint32)(p) = uint32(v)
	}), nil
}

func decodeCompileUint64(typ *rtype, structName, fieldName string) (decoder, error) {
	return newUintDecoder(typ, structName, fieldName, func(p unsafe.Pointer, v uint64) {
		*(*uint64)(p) = v
	}), nil
}

func decodeCompileFloat32(structName, fieldName string) (decoder, error) {
	return newFloatDecoder(structName, fieldName, func(p unsafe.Pointer, v float64) {
		*(*float32)(p) = float32(v)
	}), nil
}

func decodeCompileFloat64(structName, fieldName string) (decoder, error) {
	return newFloatDecoder(structName, fieldName, func(p unsafe.Pointer, v float64) {
		*(*float64)(p) = v
	}), nil
}

func decodeCompileString(typ *rtype, structName, fieldName string) (decoder, error) {
	if typ == type2rtype(jsonNumberType) {
		return newNumberDecoder(structName, fieldName, func(p unsafe.Pointer, v Number) {
			*(*Number)(p) = v
		}), nil
	}
	return newStringDecoder(structName, fieldName), nil
}

func decodeCompileBool(structName, fieldName string) (decoder, error) {
	return newBoolDecoder(structName, fieldName), nil
}

func decodeCompileBytes(typ *rtype, structName, fieldName string) (decoder, error) {
	return newBytesDecoder(typ, structName, fieldName), nil
}

func decodeCompileSlice(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	elem := typ.Elem()
	decoder, err := decodeCompile(elem, structName, fieldName, structTypeToDecoder)
	if err != nil {
		return nil, err
	}
	return newSliceDecoder(decoder, elem, elem.Size(), structName, fieldName), nil
}

func decodeCompileArray(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	elem := typ.Elem()
	decoder, err := decodeCompile(elem, structName, fieldName, structTypeToDecoder)
	if err != nil {
		return nil, err
	}
	return newArrayDecoder(decoder, elem, typ.Len(), structName, fieldName), nil
}

func decodeCompileMap(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	keyDec, err := decodeCompileMapKey(typ.Key(), structName, fieldName, structTypeToDecoder)
	if err != nil {
		return nil, err
	}
	valueDec, err := decodeCompile(typ.Elem(), structName, fieldName, structTypeToDecoder)
	if err != nil {
		return nil, err
	}
	return newMapDecoder(typ, typ.Key(), keyDec, typ.Elem(), valueDec, structName, fieldName), nil
}

func decodeCompileInterface(typ *rtype, structName, fieldName string) (decoder, error) {
	return newInterfaceDecoder(typ, structName, fieldName), nil
}

func decodeRemoveConflictFields(fieldMap map[string]*structFieldSet, conflictedMap map[string]struct{}, dec *structDecoder, field reflect.StructField) {
	for k, v := range dec.fieldMap {
		if _, exists := conflictedMap[k]; exists {
			// already conflicted key
			continue
		}
		set, exists := fieldMap[k]
		if !exists {
			fieldSet := &structFieldSet{
				dec:         v.dec,
				offset:      field.Offset + v.offset,
				isTaggedKey: v.isTaggedKey,
				key:         k,
				keyLen:      int64(len(k)),
			}
			fieldMap[k] = fieldSet
			lower := strings.ToLower(k)
			if _, exists := fieldMap[lower]; !exists {
				fieldMap[lower] = fieldSet
			}
			continue
		}
		if set.isTaggedKey {
			if v.isTaggedKey {
				// conflict tag key
				delete(fieldMap, k)
				delete(fieldMap, strings.ToLower(k))
				conflictedMap[k] = struct{}{}
				conflictedMap[strings.ToLower(k)] = struct{}{}
			}
		} else {
			if v.isTaggedKey {
				fieldSet := &structFieldSet{
					dec:         v.dec,
					offset:      field.Offset + v.offset,
					isTaggedKey: v.isTaggedKey,
					key:         k,
					keyLen:      int64(len(k)),
				}
				fieldMap[k] = fieldSet
				lower := strings.ToLower(k)
				if _, exists := fieldMap[lower]; !exists {
					fieldMap[lower] = fieldSet
				}
			} else {
				// conflict tag key
				delete(fieldMap, k)
				delete(fieldMap, strings.ToLower(k))
				conflictedMap[k] = struct{}{}
				conflictedMap[strings.ToLower(k)] = struct{}{}
			}
		}
	}
}

func decodeCompileStruct(typ *rtype, structName, fieldName string, structTypeToDecoder map[uintptr]decoder) (decoder, error) {
	fieldNum := typ.NumField()
	conflictedMap := map[string]struct{}{}
	fieldMap := map[string]*structFieldSet{}
	typeptr := uintptr(unsafe.Pointer(typ))
	if dec, exists := structTypeToDecoder[typeptr]; exists {
		return dec, nil
	}
	structDec := newStructDecoder(structName, fieldName, fieldMap)
	structTypeToDecoder[typeptr] = structDec
	structName = typ.Name()
	for i := 0; i < fieldNum; i++ {
		field := typ.Field(i)
		if runtime.IsIgnoredStructField(field) {
			continue
		}
		isUnexportedField := unicode.IsLower([]rune(field.Name)[0])
		tag := runtime.StructTagFromField(field)
		dec, err := decodeCompile(type2rtype(field.Type), structName, field.Name, structTypeToDecoder)
		if err != nil {
			return nil, err
		}
		if field.Anonymous && !tag.IsTaggedKey {
			if stDec, ok := dec.(*structDecoder); ok {
				if type2rtype(field.Type) == typ {
					// recursive definition
					continue
				}
				decodeRemoveConflictFields(fieldMap, conflictedMap, stDec, field)
			} else if pdec, ok := dec.(*ptrDecoder); ok {
				contentDec := pdec.contentDecoder()
				if pdec.typ == typ {
					// recursive definition
					continue
				}
				var fieldSetErr error
				if isUnexportedField {
					fieldSetErr = fmt.Errorf(
						"json: cannot set embedded pointer to unexported struct: %v",
						field.Type.Elem(),
					)
				}
				if dec, ok := contentDec.(*structDecoder); ok {
					for k, v := range dec.fieldMap {
						if _, exists := conflictedMap[k]; exists {
							// already conflicted key
							continue
						}
						set, exists := fieldMap[k]
						if !exists {
							fieldSet := &structFieldSet{
								dec:         newAnonymousFieldDecoder(pdec.typ, v.offset, v.dec),
								offset:      field.Offset,
								isTaggedKey: v.isTaggedKey,
								key:         k,
								keyLen:      int64(len(k)),
								err:         fieldSetErr,
							}
							fieldMap[k] = fieldSet
							lower := strings.ToLower(k)
							if _, exists := fieldMap[lower]; !exists {
								fieldMap[lower] = fieldSet
							}
							continue
						}
						if set.isTaggedKey {
							if v.isTaggedKey {
								// conflict tag key
								delete(fieldMap, k)
								delete(fieldMap, strings.ToLower(k))
								conflictedMap[k] = struct{}{}
								conflictedMap[strings.ToLower(k)] = struct{}{}
							}
						} else {
							if v.isTaggedKey {
								fieldSet := &structFieldSet{
									dec:         newAnonymousFieldDecoder(pdec.typ, v.offset, v.dec),
									offset:      field.Offset,
									isTaggedKey: v.isTaggedKey,
									key:         k,
									keyLen:      int64(len(k)),
									err:         fieldSetErr,
								}
								fieldMap[k] = fieldSet
								lower := strings.ToLower(k)
								if _, exists := fieldMap[lower]; !exists {
									fieldMap[lower] = fieldSet
								}
							} else {
								// conflict tag key
								delete(fieldMap, k)
								delete(fieldMap, strings.ToLower(k))
								conflictedMap[k] = struct{}{}
								conflictedMap[strings.ToLower(k)] = struct{}{}
							}
						}
					}
				}
			}
		} else {
			if tag.IsString && isStringTagSupportedType(type2rtype(field.Type)) {
				dec = newWrappedStringDecoder(type2rtype(field.Type), dec, structName, field.Name)
			}
			var key string
			if tag.Key != "" {
				key = tag.Key
			} else {
				key = field.Name
			}
			fieldSet := &structFieldSet{
				dec:         dec,
				offset:      field.Offset,
				isTaggedKey: tag.IsTaggedKey,
				key:         key,
				keyLen:      int64(len(key)),
			}
			fieldMap[key] = fieldSet
			lower := strings.ToLower(key)
			if _, exists := fieldMap[lower]; !exists {
				fieldMap[lower] = fieldSet
			}
		}
	}
	delete(structTypeToDecoder, typeptr)
	structDec.tryOptimize()
	return structDec, nil
}
