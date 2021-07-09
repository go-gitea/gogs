package driver

type IDB interface {
	Close() error

	Get(key []byte) ([]byte, error)

	Put(key []byte, value []byte) error
	Delete(key []byte) error

	SyncPut(key []byte, value []byte) error
	SyncDelete(key []byte) error

	NewIterator() IIterator

	NewWriteBatch() IWriteBatch

	NewSnapshot() (ISnapshot, error)

	Compact() error
}

type ISnapshot interface {
	Get(key []byte) ([]byte, error)
	NewIterator() IIterator
	Close()
}

type IIterator interface {
	Close() error

	First()
	Last()
	Seek(key []byte)

	Next()
	Prev()

	Valid() bool

	Key() []byte
	Value() []byte
}

type IWriteBatch interface {
	Put(key []byte, value []byte)
	Delete(key []byte)
	Commit() error
	SyncCommit() error
	Rollback() error
	Data() []byte
	Close()
}

type ISliceGeter interface {
	GetSlice(key []byte) (ISlice, error)
}
