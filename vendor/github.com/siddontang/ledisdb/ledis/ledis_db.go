package ledis

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/siddontang/ledisdb/store"
)

type ibucket interface {
	Get(key []byte) ([]byte, error)
	GetSlice(key []byte) (store.Slice, error)

	Put(key []byte, value []byte) error
	Delete(key []byte) error

	NewIterator() *store.Iterator

	NewWriteBatch() *store.WriteBatch

	RangeIterator(min []byte, max []byte, rangeType uint8) *store.RangeLimitIterator
	RevRangeIterator(min []byte, max []byte, rangeType uint8) *store.RangeLimitIterator
	RangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *store.RangeLimitIterator
	RevRangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *store.RangeLimitIterator
}

// DB is the database.
type DB struct {
	l *Ledis

	sdb *store.DB

	bucket ibucket

	index int

	// buffer to store index varint
	indexVarBuf []byte

	kvBatch   *batch
	listBatch *batch
	hashBatch *batch
	zsetBatch *batch
	//	binBatch  *batch
	setBatch *batch

	// status uint8

	ttlChecker *ttlChecker

	lbkeys *lBlockKeys
}

func (l *Ledis) newDB(index int) *DB {
	d := new(DB)

	d.l = l

	d.sdb = l.ldb

	d.bucket = d.sdb

	//	d.status = DBAutoCommit
	d.setIndex(index)

	d.kvBatch = d.newBatch()
	d.listBatch = d.newBatch()
	d.hashBatch = d.newBatch()
	d.zsetBatch = d.newBatch()
	// d.binBatch = d.newBatch()
	d.setBatch = d.newBatch()

	d.lbkeys = newLBlockKeys()

	d.ttlChecker = d.newTTLChecker()

	return d
}

func decodeDBIndex(buf []byte) (int, int, error) {
	index, n := binary.Uvarint(buf)
	if n == 0 {
		return 0, 0, fmt.Errorf("buf is too small to save index")
	} else if n < 0 {
		return 0, 0, fmt.Errorf("value larger than 64 bits")
	} else if index > uint64(MaxDatabases) {
		return 0, 0, fmt.Errorf("value %d is larger than max databases %d", index, MaxDatabases)
	}
	return int(index), n, nil
}

func (db *DB) setIndex(index int) {
	db.index = index
	// the most size for varint is 10 bytes
	buf := make([]byte, 10)
	n := binary.PutUvarint(buf, uint64(index))

	db.indexVarBuf = buf[0:n]
}

func (db *DB) checkKeyIndex(buf []byte) (int, error) {
	if len(buf) < len(db.indexVarBuf) {
		return 0, fmt.Errorf("key is too small")
	} else if !bytes.Equal(db.indexVarBuf, buf[0:len(db.indexVarBuf)]) {
		return 0, fmt.Errorf("invalid db index")
	}

	return len(db.indexVarBuf), nil
}

func (db *DB) newTTLChecker() *ttlChecker {
	c := new(ttlChecker)
	c.db = db
	c.txs = make([]*batch, maxDataType)
	c.cbs = make([]onExpired, maxDataType)
	c.nc = 0

	c.register(KVType, db.kvBatch, db.delete)
	c.register(ListType, db.listBatch, db.lDelete)
	c.register(HashType, db.hashBatch, db.hDelete)
	c.register(ZSetType, db.zsetBatch, db.zDelete)
	//		c.register(BitType, db.binBatch, db.bDelete)
	c.register(SetType, db.setBatch, db.sDelete)

	return c
}

func (db *DB) newBatch() *batch {
	return db.l.newBatch(db.bucket.NewWriteBatch(), &dbBatchLocker{l: &sync.Mutex{}, wrLock: &db.l.wLock})
}

// Index gets the index of database.
func (db *DB) Index() int {
	return int(db.index)
}

// func (db *DB) IsAutoCommit() bool {
// 	return db.status == DBAutoCommit
// }

// FlushAll flushes the data.
func (db *DB) FlushAll() (drop int64, err error) {
	all := [...](func() (int64, error)){
		db.flush,
		db.lFlush,
		db.hFlush,
		db.zFlush,
		db.sFlush}

	for _, flush := range all {
		n, e := flush()
		if e != nil {
			err = e
			return
		}

		drop += n
	}

	return
}

func (db *DB) flushType(t *batch, dataType byte) (drop int64, err error) {
	var deleteFunc func(t *batch, key []byte) int64
	var metaDataType byte
	switch dataType {
	case KVType:
		deleteFunc = db.delete
		metaDataType = KVType
	case ListType:
		deleteFunc = db.lDelete
		metaDataType = LMetaType
	case HashType:
		deleteFunc = db.hDelete
		metaDataType = HSizeType
	case ZSetType:
		deleteFunc = db.zDelete
		metaDataType = ZSizeType
	// case BitType:
	// 	deleteFunc = db.bDelete
	// 	metaDataType = BitMetaType
	case SetType:
		deleteFunc = db.sDelete
		metaDataType = SSizeType
	default:
		return 0, fmt.Errorf("invalid data type: %s", TypeName[dataType])
	}

	var keys [][]byte
	keys, err = db.scanGeneric(metaDataType, nil, 1024, false, "", false)
	for len(keys) != 0 || err != nil {
		for _, key := range keys {
			deleteFunc(t, key)
			db.rmExpire(t, dataType, key)

		}

		if err = t.Commit(); err != nil {
			return
		}

		drop += int64(len(keys))
		keys, err = db.scanGeneric(metaDataType, nil, 1024, false, "", false)
	}
	return
}
