package ledis

import (
	"container/list"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/siddontang/go/hack"
	"github.com/siddontang/go/log"
	"github.com/siddontang/go/num"
	"github.com/siddontang/ledisdb/store"
	"golang.org/x/net/context"
)

const (
	listHeadSeq int32 = 1
	listTailSeq int32 = 2

	listMinSeq     int32 = 1000
	listMaxSeq     int32 = 1<<31 - 1000
	listInitialSeq int32 = listMinSeq + (listMaxSeq-listMinSeq)/2
)

var errLMetaKey = errors.New("invalid lmeta key")
var errListKey = errors.New("invalid list key")
var errListSeq = errors.New("invalid list sequence, overflow")

func (db *DB) lEncodeMetaKey(key []byte) []byte {
	buf := make([]byte, len(key)+1+len(db.indexVarBuf))
	pos := copy(buf, db.indexVarBuf)
	buf[pos] = LMetaType
	pos++

	copy(buf[pos:], key)
	return buf
}

func (db *DB) lDecodeMetaKey(ek []byte) ([]byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, err
	}

	if pos+1 > len(ek) || ek[pos] != LMetaType {
		return nil, errLMetaKey
	}

	pos++
	return ek[pos:], nil
}

func (db *DB) lEncodeListKey(key []byte, seq int32) []byte {
	buf := make([]byte, len(key)+7+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)

	buf[pos] = ListType
	pos++

	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2

	copy(buf[pos:], key)
	pos += len(key)

	binary.BigEndian.PutUint32(buf[pos:], uint32(seq))

	return buf
}

func (db *DB) lDecodeListKey(ek []byte) (key []byte, seq int32, err error) {
	pos := 0
	pos, err = db.checkKeyIndex(ek)
	if err != nil {
		return
	}

	if pos+1 > len(ek) || ek[pos] != ListType {
		err = errListKey
		return
	}

	pos++

	if pos+2 > len(ek) {
		err = errListKey
		return
	}

	keyLen := int(binary.BigEndian.Uint16(ek[pos:]))
	pos += 2
	if keyLen+pos+4 != len(ek) {
		err = errListKey
		return
	}

	key = ek[pos : pos+keyLen]
	seq = int32(binary.BigEndian.Uint32(ek[pos+keyLen:]))
	return
}

func (db *DB) lpush(key []byte, whereSeq int32, args ...[]byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	var headSeq int32
	var tailSeq int32
	var size int32
	var err error

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	metaKey := db.lEncodeMetaKey(key)
	headSeq, tailSeq, size, err = db.lGetMeta(nil, metaKey)
	if err != nil {
		return 0, err
	}

	pushCnt := len(args)
	if pushCnt == 0 {
		return int64(size), nil
	}

	seq := headSeq
	var delta int32 = -1
	if whereSeq == listTailSeq {
		seq = tailSeq
		delta = 1
	}

	//	append elements
	if size > 0 {
		seq += delta
	}

	for i := 0; i < pushCnt; i++ {
		ek := db.lEncodeListKey(key, seq+int32(i)*delta)
		t.Put(ek, args[i])
	}

	seq += int32(pushCnt-1) * delta
	if seq <= listMinSeq || seq >= listMaxSeq {
		return 0, errListSeq
	}

	//	set meta info
	if whereSeq == listHeadSeq {
		headSeq = seq
	} else {
		tailSeq = seq
	}

	db.lSetMeta(metaKey, headSeq, tailSeq)

	err = t.Commit()

	if err == nil {
		db.lSignalAsReady(key)
	}

	return int64(size) + int64(pushCnt), err
}

func (db *DB) lpop(key []byte, whereSeq int32) ([]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	var headSeq int32
	var tailSeq int32
	var size int32
	var err error

	metaKey := db.lEncodeMetaKey(key)
	headSeq, tailSeq, size, err = db.lGetMeta(nil, metaKey)
	if err != nil {
		return nil, err
	} else if size == 0 {
		return nil, nil
	}

	var value []byte

	seq := headSeq
	if whereSeq == listTailSeq {
		seq = tailSeq
	}

	itemKey := db.lEncodeListKey(key, seq)
	value, err = db.bucket.Get(itemKey)
	if err != nil {
		return nil, err
	}

	if whereSeq == listHeadSeq {
		headSeq++
	} else {
		tailSeq--
	}

	t.Delete(itemKey)
	size = db.lSetMeta(metaKey, headSeq, tailSeq)
	if size == 0 {
		db.rmExpire(t, ListType, key)
	}

	err = t.Commit()
	return value, err
}

func (db *DB) ltrim2(key []byte, startP, stopP int64) (err error) {
	if err := checkKeySize(key); err != nil {
		return err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	var headSeq int32
	var llen int32
	start := int32(startP)
	stop := int32(stopP)

	ek := db.lEncodeMetaKey(key)
	if headSeq, _, llen, err = db.lGetMeta(nil, ek); err != nil {
		return err
	}

	if start < 0 {
		start = llen + start
	}
	if stop < 0 {
		stop = llen + stop
	}
	if start >= llen || start > stop {
		db.lDelete(t, key)
		db.rmExpire(t, ListType, key)
		return t.Commit()
	}

	if start < 0 {
		start = 0
	}
	if stop >= llen {
		stop = llen - 1
	}

	if start > 0 {
		for i := int32(0); i < start; i++ {
			t.Delete(db.lEncodeListKey(key, headSeq+i))
		}
	}
	if stop < int32(llen-1) {
		for i := int32(stop + 1); i < llen; i++ {
			t.Delete(db.lEncodeListKey(key, headSeq+i))
		}
	}

	db.lSetMeta(ek, headSeq+start, headSeq+stop)

	return t.Commit()
}

func (db *DB) ltrim(key []byte, trimSize, whereSeq int32) (int32, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	if trimSize == 0 {
		return 0, nil
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	var headSeq int32
	var tailSeq int32
	var size int32
	var err error

	metaKey := db.lEncodeMetaKey(key)
	headSeq, tailSeq, size, err = db.lGetMeta(nil, metaKey)
	if err != nil {
		return 0, err
	} else if size == 0 {
		return 0, nil
	}

	var (
		trimStartSeq int32
		trimEndSeq   int32
	)

	if whereSeq == listHeadSeq {
		trimStartSeq = headSeq
		trimEndSeq = num.MinInt32(trimStartSeq+trimSize-1, tailSeq)
		headSeq = trimEndSeq + 1
	} else {
		trimEndSeq = tailSeq
		trimStartSeq = num.MaxInt32(trimEndSeq-trimSize+1, headSeq)
		tailSeq = trimStartSeq - 1
	}

	for trimSeq := trimStartSeq; trimSeq <= trimEndSeq; trimSeq++ {
		itemKey := db.lEncodeListKey(key, trimSeq)
		t.Delete(itemKey)
	}

	size = db.lSetMeta(metaKey, headSeq, tailSeq)
	if size == 0 {
		db.rmExpire(t, ListType, key)
	}

	err = t.Commit()
	return trimEndSeq - trimStartSeq + 1, err
}

//	ps : here just focus on deleting the list data,
//		 any other likes expire is ignore.
func (db *DB) lDelete(t *batch, key []byte) int64 {
	mk := db.lEncodeMetaKey(key)

	var headSeq int32
	var tailSeq int32
	var err error

	it := db.bucket.NewIterator()
	defer it.Close()

	headSeq, tailSeq, _, err = db.lGetMeta(it, mk)
	if err != nil {
		return 0
	}

	var num int64
	startKey := db.lEncodeListKey(key, headSeq)
	stopKey := db.lEncodeListKey(key, tailSeq)

	rit := store.NewRangeIterator(it, &store.Range{
		Min:  startKey,
		Max:  stopKey,
		Type: store.RangeClose})
	for ; rit.Valid(); rit.Next() {
		t.Delete(rit.RawKey())
		num++
	}

	t.Delete(mk)

	return num
}

func (db *DB) lGetMeta(it *store.Iterator, ek []byte) (headSeq int32, tailSeq int32, size int32, err error) {
	var v []byte
	if it != nil {
		v = it.Find(ek)
	} else {
		v, err = db.bucket.Get(ek)
	}
	if err != nil {
		return
	} else if v == nil {
		headSeq = listInitialSeq
		tailSeq = listInitialSeq
		size = 0
		return
	} else {
		headSeq = int32(binary.LittleEndian.Uint32(v[0:4]))
		tailSeq = int32(binary.LittleEndian.Uint32(v[4:8]))
		size = tailSeq - headSeq + 1
	}
	return
}

func (db *DB) lSetMeta(ek []byte, headSeq int32, tailSeq int32) int32 {
	t := db.listBatch

	size := tailSeq - headSeq + 1
	if size < 0 {
		//	todo : log error + panic
		log.Fatalf("invalid meta sequence range [%d, %d]", headSeq, tailSeq)
	} else if size == 0 {
		t.Delete(ek)
	} else {
		buf := make([]byte, 8)

		binary.LittleEndian.PutUint32(buf[0:4], uint32(headSeq))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(tailSeq))

		t.Put(ek, buf)
	}

	return size
}

func (db *DB) lExpireAt(key []byte, when int64) (int64, error) {
	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	if llen, err := db.LLen(key); err != nil || llen == 0 {
		return 0, err
	}

	db.expireAt(t, ListType, key, when)
	if err := t.Commit(); err != nil {
		return 0, err
	}

	return 1, nil
}

// LIndex returns the value at index.
func (db *DB) LIndex(key []byte, index int32) ([]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	var seq int32
	var headSeq int32
	var tailSeq int32
	var err error

	metaKey := db.lEncodeMetaKey(key)

	it := db.bucket.NewIterator()
	defer it.Close()

	headSeq, tailSeq, _, err = db.lGetMeta(it, metaKey)
	if err != nil {
		return nil, err
	}

	if index >= 0 {
		seq = headSeq + index
	} else {
		seq = tailSeq + index + 1
	}

	sk := db.lEncodeListKey(key, seq)
	v := it.Find(sk)

	return v, nil
}

// LLen gets the length of the list.
func (db *DB) LLen(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	ek := db.lEncodeMetaKey(key)
	_, _, size, err := db.lGetMeta(nil, ek)
	return int64(size), err
}

// LPop pops the value.
func (db *DB) LPop(key []byte) ([]byte, error) {
	return db.lpop(key, listHeadSeq)
}

// LTrim trims the value from start to stop.
func (db *DB) LTrim(key []byte, start, stop int64) error {
	return db.ltrim2(key, start, stop)
}

// LTrimFront trims the value from top.
func (db *DB) LTrimFront(key []byte, trimSize int32) (int32, error) {
	return db.ltrim(key, trimSize, listHeadSeq)
}

// LTrimBack trims the value from back.
func (db *DB) LTrimBack(key []byte, trimSize int32) (int32, error) {
	return db.ltrim(key, trimSize, listTailSeq)
}

// LPush push the value to the list.
func (db *DB) LPush(key []byte, args ...[]byte) (int64, error) {
	return db.lpush(key, listHeadSeq, args...)
}

// LSet sets the value at index.
func (db *DB) LSet(key []byte, index int32, value []byte) error {
	if err := checkKeySize(key); err != nil {
		return err
	}

	var seq int32
	var headSeq int32
	var tailSeq int32
	//var size int32
	var err error
	t := db.listBatch
	t.Lock()
	defer t.Unlock()
	metaKey := db.lEncodeMetaKey(key)

	headSeq, tailSeq, _, err = db.lGetMeta(nil, metaKey)
	if err != nil {
		return err
	}

	if index >= 0 {
		seq = headSeq + index
	} else {
		seq = tailSeq + index + 1
	}
	if seq < headSeq || seq > tailSeq {
		return errListIndex
	}
	sk := db.lEncodeListKey(key, seq)
	t.Put(sk, value)
	err = t.Commit()
	return err
}

// LRange gets the value of list at range.
func (db *DB) LRange(key []byte, start int32, stop int32) ([][]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	var headSeq int32
	var llen int32
	var err error

	metaKey := db.lEncodeMetaKey(key)

	it := db.bucket.NewIterator()
	defer it.Close()

	if headSeq, _, llen, err = db.lGetMeta(it, metaKey); err != nil {
		return nil, err
	}

	if start < 0 {
		start = llen + start
	}
	if stop < 0 {
		stop = llen + stop
	}
	if start < 0 {
		start = 0
	}

	if start > stop || start >= llen {
		return [][]byte{}, nil
	}

	if stop >= llen {
		stop = llen - 1
	}

	limit := (stop - start) + 1
	headSeq += start

	v := make([][]byte, 0, limit)

	startKey := db.lEncodeListKey(key, headSeq)
	rit := store.NewRangeLimitIterator(it,
		&store.Range{
			Min:  startKey,
			Max:  nil,
			Type: store.RangeClose},
		&store.Limit{
			Offset: 0,
			Count:  int(limit)})

	for ; rit.Valid(); rit.Next() {
		v = append(v, rit.Value())
	}

	return v, nil
}

// RPop rpops the value.
func (db *DB) RPop(key []byte) ([]byte, error) {
	return db.lpop(key, listTailSeq)
}

// RPush rpushs the value .
func (db *DB) RPush(key []byte, args ...[]byte) (int64, error) {
	return db.lpush(key, listTailSeq, args...)
}

// LClear clears the list.
func (db *DB) LClear(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	num := db.lDelete(t, key)
	db.rmExpire(t, ListType, key)

	err := t.Commit()
	return num, err
}

// LMclear clears multi lists.
func (db *DB) LMclear(keys ...[]byte) (int64, error) {
	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	for _, key := range keys {
		if err := checkKeySize(key); err != nil {
			return 0, err
		}

		db.lDelete(t, key)
		db.rmExpire(t, ListType, key)

	}

	err := t.Commit()
	return int64(len(keys)), err
}

func (db *DB) lFlush() (drop int64, err error) {
	t := db.listBatch
	t.Lock()
	defer t.Unlock()
	return db.flushType(t, ListType)
}

// LExpire expires the list.
func (db *DB) LExpire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	return db.lExpireAt(key, time.Now().Unix()+duration)
}

// LExpireAt expires the list at when.
func (db *DB) LExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	return db.lExpireAt(key, when)
}

// LTTL gets the TTL of list.
func (db *DB) LTTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(ListType, key)
}

// LPersist removes the TTL of list.
func (db *DB) LPersist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.rmExpire(t, ListType, key)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

func (db *DB) lEncodeMinKey() []byte {
	return db.lEncodeMetaKey(nil)
}

func (db *DB) lEncodeMaxKey() []byte {
	ek := db.lEncodeMetaKey(nil)
	ek[len(ek)-1] = LMetaType + 1
	return ek
}

// BLPop pops the list with block way.
func (db *DB) BLPop(keys [][]byte, timeout time.Duration) ([]interface{}, error) {
	return db.lblockPop(keys, listHeadSeq, timeout)
}

// BRPop bpops the list with block way.
func (db *DB) BRPop(keys [][]byte, timeout time.Duration) ([]interface{}, error) {
	return db.lblockPop(keys, listTailSeq, timeout)
}

// LKeyExists check list existed or not.
func (db *DB) LKeyExists(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}
	sk := db.lEncodeMetaKey(key)
	v, err := db.bucket.Get(sk)
	if v != nil && err == nil {
		return 1, nil
	}
	return 0, err
}

func (db *DB) lblockPop(keys [][]byte, whereSeq int32, timeout time.Duration) ([]interface{}, error) {
	for {
		var ctx context.Context
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}

		for _, key := range keys {
			v, err := db.lbkeys.popOrWait(db, key, whereSeq, cancel)

			if err != nil {
				cancel()
				return nil, err
			} else if v != nil {
				cancel()
				return []interface{}{key, v}, nil
			}
		}

		//blocking wait
		<-ctx.Done()
		cancel()

		//if ctx.Err() is a deadline exceeded (timeout) we return
		//otherwise we try to pop one of the keys again.
		if ctx.Err() == context.DeadlineExceeded {
			return nil, nil
		}
	}
}

func (db *DB) lSignalAsReady(key []byte) {
	db.lbkeys.signal(key)
}

type lBlockKeys struct {
	sync.Mutex

	keys map[string]*list.List
}

func newLBlockKeys() *lBlockKeys {
	l := new(lBlockKeys)

	l.keys = make(map[string]*list.List)
	return l
}

func (l *lBlockKeys) signal(key []byte) {
	l.Lock()
	defer l.Unlock()

	s := hack.String(key)
	fns, ok := l.keys[s]
	if !ok {
		return
	}
	for e := fns.Front(); e != nil; e = e.Next() {
		fn := e.Value.(context.CancelFunc)
		fn()
	}

	delete(l.keys, s)
}

func (l *lBlockKeys) popOrWait(db *DB, key []byte, whereSeq int32, fn context.CancelFunc) ([]interface{}, error) {
	v, err := db.lpop(key, whereSeq)
	if err != nil {
		return nil, err
	} else if v != nil {
		return []interface{}{key, v}, nil
	}

	l.Lock()

	s := hack.String(key)
	chs, ok := l.keys[s]
	if !ok {
		chs = list.New()
		l.keys[s] = chs
	}

	chs.PushBack(fn)
	l.Unlock()
	return nil, nil
}
