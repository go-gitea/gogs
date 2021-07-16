package funk

import (
	"fmt"
	"reflect"
)

type chainBuilder struct {
	collection interface{}
}

func (b *chainBuilder) Chunk(size int) Builder {
	return &chainBuilder{Chunk(b.collection, size)}
}
func (b *chainBuilder) Compact() Builder {
	return &chainBuilder{Compact(b.collection)}
}
func (b *chainBuilder) Drop(n int) Builder {
	return &chainBuilder{Drop(b.collection, n)}
}
func (b *chainBuilder) Filter(predicate interface{}) Builder {
	return &chainBuilder{Filter(b.collection, predicate)}
}
func (b *chainBuilder) Flatten() Builder {
	return &chainBuilder{Flatten(b.collection)}
}
func (b *chainBuilder) FlattenDeep() Builder {
	return &chainBuilder{FlattenDeep(b.collection)}
}
func (b *chainBuilder) Initial() Builder {
	return &chainBuilder{Initial(b.collection)}
}
func (b *chainBuilder) Intersect(y interface{}) Builder {
	return &chainBuilder{Intersect(b.collection, y)}
}
func (b *chainBuilder) Join(rarr interface{}, fnc JoinFnc) Builder {
	return &chainBuilder{Join(b.collection, rarr, fnc)}
}
func (b *chainBuilder) Map(mapFunc interface{}) Builder {
	return &chainBuilder{Map(b.collection, mapFunc)}
}
func (b *chainBuilder) FlatMap(mapFunc interface{}) Builder {
	return &chainBuilder{FlatMap(b.collection, mapFunc)}
}
func (b *chainBuilder) Reverse() Builder {
	return &chainBuilder{Reverse(b.collection)}
}
func (b *chainBuilder) Shuffle() Builder {
	return &chainBuilder{Shuffle(b.collection)}
}
func (b *chainBuilder) Tail() Builder {
	return &chainBuilder{Tail(b.collection)}
}
func (b *chainBuilder) Uniq() Builder {
	return &chainBuilder{Uniq(b.collection)}
}
func (b *chainBuilder) Without(values ...interface{}) Builder {
	return &chainBuilder{Without(b.collection, values...)}
}

func (b *chainBuilder) All() bool {
	v := reflect.ValueOf(b.collection)
	t := v.Type()

	if t.Kind() != reflect.Array && t.Kind() != reflect.Slice {
		panic(fmt.Sprintf("Type %s is not supported by Chain.All", t.String()))
	}

	c := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		c[i] = v.Index(i).Interface()
	}
	return All(c...)
}
func (b *chainBuilder) Any() bool {
	v := reflect.ValueOf(b.collection)
	t := v.Type()

	if t.Kind() != reflect.Array && t.Kind() != reflect.Slice {
		panic(fmt.Sprintf("Type %s is not supported by Chain.Any", t.String()))
	}

	c := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		c[i] = v.Index(i).Interface()
	}
	return Any(c...)
}
func (b *chainBuilder) Contains(elem interface{}) bool {
	return Contains(b.collection, elem)
}
func (b *chainBuilder) Every(elements ...interface{}) bool {
	return Every(b.collection, elements...)
}
func (b *chainBuilder) Find(predicate interface{}) interface{} {
	return Find(b.collection, predicate)
}
func (b *chainBuilder) ForEach(predicate interface{}) {
	ForEach(b.collection, predicate)
}
func (b *chainBuilder) ForEachRight(predicate interface{}) {
	ForEachRight(b.collection, predicate)
}
func (b *chainBuilder) Head() interface{} {
	return Head(b.collection)
}
func (b *chainBuilder) Keys() interface{} {
	return Keys(b.collection)
}
func (b *chainBuilder) IndexOf(elem interface{}) int {
	return IndexOf(b.collection, elem)
}
func (b *chainBuilder) IsEmpty() bool {
	return IsEmpty(b.collection)
}
func (b *chainBuilder) Last() interface{} {
	return Last(b.collection)
}
func (b *chainBuilder) LastIndexOf(elem interface{}) int {
	return LastIndexOf(b.collection, elem)
}
func (b *chainBuilder) NotEmpty() bool {
	return NotEmpty(b.collection)
}
func (b *chainBuilder) Product() float64 {
	return Product(b.collection)
}
func (b *chainBuilder) Reduce(reduceFunc, acc interface{}) interface{} {
	return Reduce(b.collection, reduceFunc, acc)
}
func (b *chainBuilder) Sum() float64 {
	return Sum(b.collection)
}
func (b *chainBuilder) Type() reflect.Type {
	return reflect.TypeOf(b.collection)
}
func (b *chainBuilder) Value() interface{} {
	return b.collection
}
func (b *chainBuilder) Values() interface{} {
	return Values(b.collection)
}
