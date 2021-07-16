package funk

import "reflect"

type lazyBuilder struct {
	exec func() interface{}
}

func (b *lazyBuilder) Chunk(size int) Builder {
	return &lazyBuilder{func() interface{} { return Chunk(b.exec(), size) }}
}
func (b *lazyBuilder) Compact() Builder {
	return &lazyBuilder{func() interface{} { return Compact(b.exec()) }}
}
func (b *lazyBuilder) Drop(n int) Builder {
	return &lazyBuilder{func() interface{} { return Drop(b.exec(), n) }}
}
func (b *lazyBuilder) Filter(predicate interface{}) Builder {
	return &lazyBuilder{func() interface{} { return Filter(b.exec(), predicate) }}
}
func (b *lazyBuilder) Flatten() Builder {
	return &lazyBuilder{func() interface{} { return Flatten(b.exec()) }}
}
func (b *lazyBuilder) FlattenDeep() Builder {
	return &lazyBuilder{func() interface{} { return FlattenDeep(b.exec()) }}
}
func (b *lazyBuilder) Initial() Builder {
	return &lazyBuilder{func() interface{} { return Initial(b.exec()) }}
}
func (b *lazyBuilder) Intersect(y interface{}) Builder {
	return &lazyBuilder{func() interface{} { return Intersect(b.exec(), y) }}
}
func (b *lazyBuilder) Join(rarr interface{}, fnc JoinFnc) Builder {
	return &lazyBuilder{func() interface{} { return Join(b.exec(), rarr, fnc) }}
}
func (b *lazyBuilder) Map(mapFunc interface{}) Builder {
	return &lazyBuilder{func() interface{} { return Map(b.exec(), mapFunc) }}
}
func (b *lazyBuilder) FlatMap(mapFunc interface{}) Builder {
	return &lazyBuilder{func() interface{} { return FlatMap(b.exec(), mapFunc) }}
}
func (b *lazyBuilder) Reverse() Builder {
	return &lazyBuilder{func() interface{} { return Reverse(b.exec()) }}
}
func (b *lazyBuilder) Shuffle() Builder {
	return &lazyBuilder{func() interface{} { return Shuffle(b.exec()) }}
}
func (b *lazyBuilder) Tail() Builder {
	return &lazyBuilder{func() interface{} { return Tail(b.exec()) }}
}
func (b *lazyBuilder) Uniq() Builder {
	return &lazyBuilder{func() interface{} { return Uniq(b.exec()) }}
}
func (b *lazyBuilder) Without(values ...interface{}) Builder {
	return &lazyBuilder{func() interface{} { return Without(b.exec(), values...) }}
}

func (b *lazyBuilder) All() bool {
	return (&chainBuilder{b.exec()}).All()
}
func (b *lazyBuilder) Any() bool {
	return (&chainBuilder{b.exec()}).Any()
}
func (b *lazyBuilder) Contains(elem interface{}) bool {
	return Contains(b.exec(), elem)
}
func (b *lazyBuilder) Every(elements ...interface{}) bool {
	return Every(b.exec(), elements...)
}
func (b *lazyBuilder) Find(predicate interface{}) interface{} {
	return Find(b.exec(), predicate)
}
func (b *lazyBuilder) ForEach(predicate interface{}) {
	ForEach(b.exec(), predicate)
}
func (b *lazyBuilder) ForEachRight(predicate interface{}) {
	ForEachRight(b.exec(), predicate)
}
func (b *lazyBuilder) Head() interface{} {
	return Head(b.exec())
}
func (b *lazyBuilder) Keys() interface{} {
	return Keys(b.exec())
}
func (b *lazyBuilder) IndexOf(elem interface{}) int {
	return IndexOf(b.exec(), elem)
}
func (b *lazyBuilder) IsEmpty() bool {
	return IsEmpty(b.exec())
}
func (b *lazyBuilder) Last() interface{} {
	return Last(b.exec())
}
func (b *lazyBuilder) LastIndexOf(elem interface{}) int {
	return LastIndexOf(b.exec(), elem)
}
func (b *lazyBuilder) NotEmpty() bool {
	return NotEmpty(b.exec())
}
func (b *lazyBuilder) Product() float64 {
	return Product(b.exec())
}
func (b *lazyBuilder) Reduce(reduceFunc, acc interface{}) interface{} {
	return Reduce(b.exec(), reduceFunc, acc)
}
func (b *lazyBuilder) Sum() float64 {
	return Sum(b.exec())
}
func (b *lazyBuilder) Type() reflect.Type {
	return reflect.TypeOf(b.exec())
}
func (b *lazyBuilder) Value() interface{} {
	return b.exec()
}
func (b *lazyBuilder) Values() interface{} {
	return Values(b.exec())
}
