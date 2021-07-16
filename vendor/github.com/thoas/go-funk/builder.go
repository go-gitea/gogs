package funk

import (
	"fmt"
	"reflect"
)

// Builder contains all tools which can be chained.
type Builder interface {
	Chunk(size int) Builder
	Compact() Builder
	Drop(n int) Builder
	Filter(predicate interface{}) Builder
	Flatten() Builder
	FlattenDeep() Builder
	Initial() Builder
	Intersect(y interface{}) Builder
	Join(rarr interface{}, fnc JoinFnc) Builder
	Map(mapFunc interface{}) Builder
	FlatMap(mapFunc interface{}) Builder
	Reverse() Builder
	Shuffle() Builder
	Tail() Builder
	Uniq() Builder
	Without(values ...interface{}) Builder

	All() bool
	Any() bool
	Contains(elem interface{}) bool
	Every(elements ...interface{}) bool
	Find(predicate interface{}) interface{}
	ForEach(predicate interface{})
	ForEachRight(predicate interface{})
	Head() interface{}
	Keys() interface{}
	IndexOf(elem interface{}) int
	IsEmpty() bool
	Last() interface{}
	LastIndexOf(elem interface{}) int
	NotEmpty() bool
	Product() float64
	Reduce(reduceFunc, acc interface{}) interface{}
	Sum() float64
	Type() reflect.Type
	Value() interface{}
	Values() interface{}
}

// Chain creates a simple new go-funk.Builder from a collection. Each method 
// call generate a new builder containing the previous result.
func Chain(v interface{}) Builder {
	isNotNil(v, "Chain")

	valueType := reflect.TypeOf(v)
	if isValidBuilderEntry(valueType) ||
		(valueType.Kind() == reflect.Ptr && isValidBuilderEntry(valueType.Elem())) {
		return &chainBuilder{v}
	}

	panic(fmt.Sprintf("Type %s is not supported by Chain", valueType.String()))
}

// LazyChain creates a lazy go-funk.Builder from a collection. Each method call
// generate a new builder containing a method generating the previous value.
// With that, all data are only generated when we call a tailling method like All or Find.
func LazyChain(v interface{}) Builder {
	isNotNil(v, "LazyChain")

	valueType := reflect.TypeOf(v)
	if isValidBuilderEntry(valueType) ||
		(valueType.Kind() == reflect.Ptr && isValidBuilderEntry(valueType.Elem())) {
		return &lazyBuilder{func() interface{} { return v }}
	}

	panic(fmt.Sprintf("Type %s is not supported by LazyChain", valueType.String()))

}

// LazyChainWith creates a lazy go-funk.Builder from a generator. Like LazyChain, each 
// method call generate a new builder containing a method generating the previous value.
// But, instead of using a collection, it takes a generator which can generate values.
// With LazyChainWith, to can create a generic pipeline of collection transformation and, 
// throw the generator, sending different collection.
func LazyChainWith(generator func() interface{}) Builder {
	isNotNil(generator, "LazyChainWith")
	return &lazyBuilder{func() interface{} {
		isNotNil(generator, "LazyChainWith")

		v := generator()
		valueType := reflect.TypeOf(v)
		if isValidBuilderEntry(valueType) ||
			(valueType.Kind() == reflect.Ptr && isValidBuilderEntry(valueType.Elem())) {
			return v
		}

		panic(fmt.Sprintf("Type %s is not supported by LazyChainWith generator", valueType.String()))
	}}
}

func isNotNil(v interface{}, from string) {
	if v == nil {
		panic(fmt.Sprintf("nil value is not supported by %s", from))
	}
}

func isValidBuilderEntry(valueType reflect.Type) bool {
	return valueType.Kind() == reflect.Slice || valueType.Kind() == reflect.Array ||
		valueType.Kind() == reflect.Map ||
		valueType.Kind() == reflect.String
}
