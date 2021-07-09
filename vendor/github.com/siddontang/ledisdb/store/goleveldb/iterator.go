package goleveldb

import (
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

type Iterator struct {
	it iterator.Iterator
}

func (it *Iterator) Key() []byte {
	return it.it.Key()
}

func (it *Iterator) Value() []byte {
	return it.it.Value()
}

func (it *Iterator) Close() error {
	if it.it != nil {
		it.it.Release()
		it.it = nil
	}
	return nil
}

func (it *Iterator) Valid() bool {
	return it.it.Valid()
}

func (it *Iterator) Next() {
	it.it.Next()
}

func (it *Iterator) Prev() {
	it.it.Prev()
}

func (it *Iterator) First() {
	it.it.First()
}

func (it *Iterator) Last() {
	it.it.Last()
}

func (it *Iterator) Seek(key []byte) {
	it.it.Seek(key)
}
