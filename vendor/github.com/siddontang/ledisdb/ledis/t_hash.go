package ledis

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/siddontang/go/num"
	"github.com/siddontang/ledisdb/store"
)

// FVPair is the pair of field and value.
type FVPair struct {
	Field []byte
	Value []byte
}

var errHashKey = errors.New("invalid hash key")
var errHSizeKey = errors.New("invalid hsize key")

const (
	hashStartSep byte = ':'
	hashStopSep  byte = hashStartSep + 1
)

func checkHashKFSize(key []byte, field []byte) error {
	if len(key) > MaxKeySize || len(key) == 0 {
		return errKeySize
	} else if len(field) > MaxHashFieldSize || len(field) == 0 {
		return errHashFieldSize
	}
	return nil
}

func (db *DB) hEncodeSizeKey(key []byte) []byte {
	buf := make([]byte, len(key)+1+len(db.indexVarBuf))

	pos := 0
	n := copy(buf, db.indexVarBuf)

	pos += n
	buf[pos] = HSizeType

	pos++
	copy(buf[pos:], key)

	return buf
}

func (db *DB) hDecodeSizeKey(ek []byte) ([]byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, err
	}

	if pos+1 > len(ek) || ek[pos] != HSizeType {
		return nil, errHSizeKey
	}
	pos++

	return ek[pos:], nil
}

func (db *DB) hEncodeHashKey(key []byte, field []byte) []byte {
	buf := make([]byte, len(key)+len(field)+1+1+2+len(db.indexVarBuf))

	pos := 0
	n := copy(buf, db.indexVarBuf)
	pos += n

	buf[pos] = HashType
	pos++

	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2

	copy(buf[pos:], key)
	pos += len(key)

	buf[pos] = hashStartSep
	pos++
	copy(buf[pos:], field)

	return buf
}

func (db *DB) hDecodeHashKey(ek []byte) ([]byte, []byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, nil, err
	}

	if pos+1 > len(ek) || ek[pos] != HashType {
		return nil, nil, errHashKey
	}
	pos++

	if pos+2 > len(ek) {
		return nil, nil, errHashKey
	}

	keyLen := int(binary.BigEndian.Uint16(ek[pos:]))
	pos += 2

	if keyLen+pos > len(ek) {
		return nil, nil, errHashKey
	}

	key := ek[pos : pos+keyLen]
	pos += keyLen

	if ek[pos] != hashStartSep {
		return nil, nil, errHashKey
	}

	pos++
	field := ek[pos:]
	return key, field, nil
}

func (db *DB) hEncodeStartKey(key []byte) []byte {
	return db.hEncodeHashKey(key, nil)
}

func (db *DB) hEncodeStopKey(key []byte) []byte {
	k := db.hEncodeHashKey(key, nil)

	k[len(k)-1] = hashStopSep

	return k
}

func (db *DB) hSetItem(key []byte, field []byte, value []byte) (int64, error) {
	t := db.hashBatch

	ek := db.hEncodeHashKey(key, field)

	var n int64 = 1
	if v, _ := db.bucket.Get(ek); v != nil {
		n = 0
	} else {
		if _, err := db.hIncrSize(key, 1); err != nil {
			return 0, err
		}
	}

	t.Put(ek, value)
	return n, nil
}

//	ps : here just focus on deleting the hash data,
//		 any other likes expire is ignore.
func (db *DB) hDelete(t *batch, key []byte) int64 {
	sk := db.hEncodeSizeKey(key)
	start := db.hEncodeStartKey(key)
	stop := db.hEncodeStopKey(key)

	var num int64
	it := db.bucket.RangeLimitIterator(start, stop, store.RangeROpen, 0, -1)
	for ; it.Valid(); it.Next() {
		t.Delete(it.Key())
		num++
	}
	it.Close()

	t.Delete(sk)
	return num
}

func (db *DB) hExpireAt(key []byte, when int64) (int64, error) {
	t := db.hashBatch
	t.Lock()
	defer t.Unlock()

	if hlen, err := db.HLen(key); err != nil || hlen == 0 {
		return 0, err
	}

	db.expireAt(t, HashType, key, when)
	if err := t.Commit(); err != nil {
		return 0, err
	}

	return 1, nil
}

// HLen returns the lengh of hash.
func (db *DB) HLen(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	return Int64(db.bucket.Get(db.hEncodeSizeKey(key)))
}

// HSet sets the field with value of key.
func (db *DB) HSet(key []byte, field []byte, value []byte) (int64, error) {
	if err := checkHashKFSize(key, field); err != nil {
		return 0, err
	} else if err := checkValueSize(value); err != nil {
		return 0, err
	}

	t := db.hashBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.hSetItem(key, field, value)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

// HGet gets the value of the field.
func (db *DB) HGet(key []byte, field []byte) ([]byte, error) {
	if err := checkHashKFSize(key, field); err != nil {
		return nil, err
	}

	return db.bucket.Get(db.hEncodeHashKey(key, field))
}

// HMset sets multi field-values.
func (db *DB) HMset(key []byte, args ...FVPair) error {
	t := db.hashBatch
	t.Lock()
	defer t.Unlock()

	var err error
	var ek []byte
	var num int64
	for i := 0; i < len(args); i++ {
		if err := checkHashKFSize(key, args[i].Field); err != nil {
			return err
		} else if err := checkValueSize(args[i].Value); err != nil {
			return err
		}

		ek = db.hEncodeHashKey(key, args[i].Field)

		if v, err := db.bucket.Get(ek); err != nil {
			return err
		} else if v == nil {
			num++
		}

		t.Put(ek, args[i].Value)
	}

	if _, err = db.hIncrSize(key, num); err != nil {
		return err
	}

	//todo add binglog
	err = t.Commit()
	return err
}

// HMget gets multi values of fields
func (db *DB) HMget(key []byte, args ...[]byte) ([][]byte, error) {
	var ek []byte

	it := db.bucket.NewIterator()
	defer it.Close()

	r := make([][]byte, len(args))
	for i := 0; i < len(args); i++ {
		if err := checkHashKFSize(key, args[i]); err != nil {
			return nil, err
		}

		ek = db.hEncodeHashKey(key, args[i])

		r[i] = it.Find(ek)
	}

	return r, nil
}

// HDel deletes the fields.
func (db *DB) HDel(key []byte, args ...[]byte) (int64, error) {
	t := db.hashBatch

	var ek []byte
	var v []byte
	var err error

	t.Lock()
	defer t.Unlock()

	it := db.bucket.NewIterator()
	defer it.Close()

	var num int64
	for i := 0; i < len(args); i++ {
		if err := checkHashKFSize(key, args[i]); err != nil {
			return 0, err
		}

		ek = db.hEncodeHashKey(key, args[i])

		v = it.RawFind(ek)
		if v == nil {
			continue
		} else {
			num++
			t.Delete(ek)
		}
	}

	if _, err = db.hIncrSize(key, -num); err != nil {
		return 0, err
	}

	err = t.Commit()

	return num, err
}

func (db *DB) hIncrSize(key []byte, delta int64) (int64, error) {
	t := db.hashBatch
	sk := db.hEncodeSizeKey(key)

	var err error
	var size int64
	if size, err = Int64(db.bucket.Get(sk)); err != nil {
		return 0, err
	}

	size += delta
	if size <= 0 {
		size = 0
		t.Delete(sk)
		db.rmExpire(t, HashType, key)
	} else {
		t.Put(sk, PutInt64(size))
	}

	return size, nil
}

// HIncrBy increases the value of field by delta.
func (db *DB) HIncrBy(key []byte, field []byte, delta int64) (int64, error) {
	if err := checkHashKFSize(key, field); err != nil {
		return 0, err
	}

	t := db.hashBatch
	var ek []byte
	var err error

	t.Lock()
	defer t.Unlock()

	ek = db.hEncodeHashKey(key, field)

	var n int64
	if n, err = StrInt64(db.bucket.Get(ek)); err != nil {
		return 0, err
	}

	n += delta

	_, err = db.hSetItem(key, field, num.FormatInt64ToSlice(n))
	if err != nil {
		return 0, err
	}

	err = t.Commit()

	return n, err
}

// HGetAll returns all field-values.
func (db *DB) HGetAll(key []byte) ([]FVPair, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	start := db.hEncodeStartKey(key)
	stop := db.hEncodeStopKey(key)

	v := make([]FVPair, 0, 16)

	it := db.bucket.RangeLimitIterator(start, stop, store.RangeROpen, 0, -1)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		_, f, err := db.hDecodeHashKey(it.Key())
		if err != nil {
			return nil, err
		}

		v = append(v, FVPair{Field: f, Value: it.Value()})
	}

	return v, nil
}

// HKeys returns the all fields.
func (db *DB) HKeys(key []byte) ([][]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	start := db.hEncodeStartKey(key)
	stop := db.hEncodeStopKey(key)

	v := make([][]byte, 0, 16)

	it := db.bucket.RangeLimitIterator(start, stop, store.RangeROpen, 0, -1)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		_, f, err := db.hDecodeHashKey(it.Key())
		if err != nil {
			return nil, err
		}
		v = append(v, f)
	}

	return v, nil
}

// HValues returns all values
func (db *DB) HValues(key []byte) ([][]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	start := db.hEncodeStartKey(key)
	stop := db.hEncodeStopKey(key)

	v := make([][]byte, 0, 16)

	it := db.bucket.RangeLimitIterator(start, stop, store.RangeROpen, 0, -1)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		_, _, err := db.hDecodeHashKey(it.Key())
		if err != nil {
			return nil, err
		}

		v = append(v, it.Value())
	}

	return v, nil
}

// HClear clears the data.
func (db *DB) HClear(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.hashBatch
	t.Lock()
	defer t.Unlock()

	num := db.hDelete(t, key)
	db.rmExpire(t, HashType, key)

	err := t.Commit()
	return num, err
}

// HMclear cleans multi data.
func (db *DB) HMclear(keys ...[]byte) (int64, error) {
	t := db.hashBatch
	t.Lock()
	defer t.Unlock()

	for _, key := range keys {
		if err := checkKeySize(key); err != nil {
			return 0, err
		}

		db.hDelete(t, key)
		db.rmExpire(t, HashType, key)
	}

	err := t.Commit()
	return int64(len(keys)), err
}

func (db *DB) hFlush() (drop int64, err error) {
	t := db.hashBatch

	t.Lock()
	defer t.Unlock()

	return db.flushType(t, HashType)
}

// HExpire expires the data with duration.
func (db *DB) HExpire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	return db.hExpireAt(key, time.Now().Unix()+duration)
}

// HExpireAt expires the data at time when.
func (db *DB) HExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	return db.hExpireAt(key, when)
}

// HTTL gets the TTL of data.
func (db *DB) HTTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(HashType, key)
}

// HPersist removes the TTL of data.
func (db *DB) HPersist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.hashBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.rmExpire(t, HashType, key)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

// HKeyExists checks whether data exists or not.
func (db *DB) HKeyExists(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}
	sk := db.hEncodeSizeKey(key)
	v, err := db.bucket.Get(sk)
	if v != nil && err == nil {
		return 1, nil
	}
	return 0, err
}
