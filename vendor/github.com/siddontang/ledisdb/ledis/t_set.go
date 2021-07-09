package ledis

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/siddontang/go/hack"
	"github.com/siddontang/ledisdb/store"
)

var errSetKey = errors.New("invalid set key")
var errSSizeKey = errors.New("invalid ssize key")

// For set operation type.
const (
	setStartSep byte = ':'
	setStopSep  byte = setStartSep + 1
	UnionType   byte = 51
	DiffType    byte = 52
	InterType   byte = 53
)

func checkSetKMSize(key []byte, member []byte) error {
	if len(key) > MaxKeySize || len(key) == 0 {
		return errKeySize
	} else if len(member) > MaxSetMemberSize || len(member) == 0 {
		return errSetMemberSize
	}
	return nil
}

func (db *DB) sEncodeSizeKey(key []byte) []byte {
	buf := make([]byte, len(key)+1+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)
	buf[pos] = SSizeType

	pos++

	copy(buf[pos:], key)
	return buf
}

func (db *DB) sDecodeSizeKey(ek []byte) ([]byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, err
	}

	if pos+1 > len(ek) || ek[pos] != SSizeType {
		return nil, errSSizeKey
	}
	pos++

	return ek[pos:], nil
}

func (db *DB) sEncodeSetKey(key []byte, member []byte) []byte {
	buf := make([]byte, len(key)+len(member)+1+1+2+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)

	buf[pos] = SetType
	pos++

	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2

	copy(buf[pos:], key)
	pos += len(key)

	buf[pos] = setStartSep
	pos++
	copy(buf[pos:], member)

	return buf
}

func (db *DB) sDecodeSetKey(ek []byte) ([]byte, []byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, nil, err
	}

	if pos+1 > len(ek) || ek[pos] != SetType {
		return nil, nil, errSetKey
	}

	pos++

	if pos+2 > len(ek) {
		return nil, nil, errSetKey
	}

	keyLen := int(binary.BigEndian.Uint16(ek[pos:]))
	pos += 2

	if keyLen+pos > len(ek) {
		return nil, nil, errSetKey
	}

	key := ek[pos : pos+keyLen]
	pos += keyLen

	if ek[pos] != hashStartSep {
		return nil, nil, errSetKey
	}

	pos++
	member := ek[pos:]
	return key, member, nil
}

func (db *DB) sEncodeStartKey(key []byte) []byte {
	return db.sEncodeSetKey(key, nil)
}

func (db *DB) sEncodeStopKey(key []byte) []byte {
	k := db.sEncodeSetKey(key, nil)

	k[len(k)-1] = setStopSep

	return k
}

func (db *DB) sFlush() (drop int64, err error) {

	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	return db.flushType(t, SetType)
}

func (db *DB) sDelete(t *batch, key []byte) int64 {
	sk := db.sEncodeSizeKey(key)
	start := db.sEncodeStartKey(key)
	stop := db.sEncodeStopKey(key)

	var num int64
	it := db.bucket.RangeLimitIterator(start, stop, store.RangeROpen, 0, -1)
	for ; it.Valid(); it.Next() {
		t.Delete(it.RawKey())
		num++
	}

	it.Close()

	t.Delete(sk)
	return num
}

func (db *DB) sIncrSize(key []byte, delta int64) (int64, error) {
	t := db.setBatch
	sk := db.sEncodeSizeKey(key)

	var err error
	var size int64
	if size, err = Int64(db.bucket.Get(sk)); err != nil {
		return 0, err
	}

	size += delta
	if size <= 0 {
		size = 0
		t.Delete(sk)
		db.rmExpire(t, SetType, key)
	} else {
		t.Put(sk, PutInt64(size))
	}

	return size, nil
}

func (db *DB) sExpireAt(key []byte, when int64) (int64, error) {
	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	if scnt, err := db.SCard(key); err != nil || scnt == 0 {
		return 0, err
	}
	db.expireAt(t, SetType, key, when)
	if err := t.Commit(); err != nil {
		return 0, err
	}

	return 1, nil
}

func (db *DB) sSetItem(key []byte, member []byte) (int64, error) {
	t := db.setBatch
	ek := db.sEncodeSetKey(key, member)

	var n int64 = 1
	if v, _ := db.bucket.Get(ek); v != nil {
		n = 0
	} else {
		if _, err := db.sIncrSize(key, 1); err != nil {
			return 0, err
		}
	}

	t.Put(ek, nil)
	return n, nil
}

// SAdd adds the value to the set.
func (db *DB) SAdd(key []byte, args ...[]byte) (int64, error) {
	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	var err error
	var ek []byte
	var num int64
	for i := 0; i < len(args); i++ {
		if err := checkSetKMSize(key, args[i]); err != nil {
			return 0, err
		}

		ek = db.sEncodeSetKey(key, args[i])

		if v, err := db.bucket.Get(ek); err != nil {
			return 0, err
		} else if v == nil {
			num++
		}

		t.Put(ek, nil)
	}

	if _, err = db.sIncrSize(key, num); err != nil {
		return 0, err
	}

	err = t.Commit()
	return num, err

}

// SCard gets the size of set.
func (db *DB) SCard(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	sk := db.sEncodeSizeKey(key)

	return Int64(db.bucket.Get(sk))
}

func (db *DB) sDiffGeneric(keys ...[]byte) ([][]byte, error) {
	destMap := make(map[string]bool)

	members, err := db.SMembers(keys[0])
	if err != nil {
		return nil, err
	}

	for _, m := range members {
		destMap[hack.String(m)] = true
	}

	for _, k := range keys[1:] {
		members, err := db.SMembers(k)
		if err != nil {
			return nil, err
		}

		for _, m := range members {
			if _, ok := destMap[hack.String(m)]; !ok {
				continue
			} else if ok {
				delete(destMap, hack.String(m))
			}
		}
		// O - A = O, O is zero set.
		if len(destMap) == 0 {
			return nil, nil
		}
	}

	slice := make([][]byte, len(destMap))
	idx := 0
	for k, v := range destMap {
		if !v {
			continue
		}
		slice[idx] = []byte(k)
		idx++
	}

	return slice, nil
}

// SDiff gets the different of sets.
func (db *DB) SDiff(keys ...[]byte) ([][]byte, error) {
	v, err := db.sDiffGeneric(keys...)
	return v, err
}

// SDiffStore gets the different of sets and stores to dest set.
func (db *DB) SDiffStore(dstKey []byte, keys ...[]byte) (int64, error) {
	n, err := db.sStoreGeneric(dstKey, DiffType, keys...)
	return n, err
}

// SKeyExists checks whether set existed or not.
func (db *DB) SKeyExists(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}
	sk := db.sEncodeSizeKey(key)
	v, err := db.bucket.Get(sk)
	if v != nil && err == nil {
		return 1, nil
	}
	return 0, err
}

func (db *DB) sInterGeneric(keys ...[]byte) ([][]byte, error) {
	destMap := make(map[string]bool)

	members, err := db.SMembers(keys[0])
	if err != nil {
		return nil, err
	}

	for _, m := range members {
		destMap[hack.String(m)] = true
	}

	for _, key := range keys[1:] {
		if err := checkKeySize(key); err != nil {
			return nil, err
		}

		members, err := db.SMembers(key)
		if err != nil {
			return nil, err
		} else if len(members) == 0 {
			return nil, err
		}

		tempMap := make(map[string]bool)
		for _, member := range members {
			if err := checkKeySize(member); err != nil {
				return nil, err
			}
			if _, ok := destMap[hack.String(member)]; ok {
				tempMap[hack.String(member)] = true //mark this item as selected
			}
		}
		destMap = tempMap //reduce the size of the result set
		if len(destMap) == 0 {
			return nil, nil
		}
	}

	slice := make([][]byte, len(destMap))
	idx := 0
	for k, v := range destMap {
		if !v {
			continue
		}

		slice[idx] = []byte(k)
		idx++
	}

	return slice, nil

}

// SInter intersects the sets.
func (db *DB) SInter(keys ...[]byte) ([][]byte, error) {
	v, err := db.sInterGeneric(keys...)
	return v, err

}

// SInterStore intersects the sets and stores to dest set.
func (db *DB) SInterStore(dstKey []byte, keys ...[]byte) (int64, error) {
	n, err := db.sStoreGeneric(dstKey, InterType, keys...)
	return n, err
}

// SIsMember checks member in set.
func (db *DB) SIsMember(key []byte, member []byte) (int64, error) {
	ek := db.sEncodeSetKey(key, member)

	var n int64 = 1
	if v, err := db.bucket.Get(ek); err != nil {
		return 0, err
	} else if v == nil {
		n = 0
	}
	return n, nil
}

// SMembers gets members of set.
func (db *DB) SMembers(key []byte) ([][]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	start := db.sEncodeStartKey(key)
	stop := db.sEncodeStopKey(key)

	v := make([][]byte, 0, 16)

	it := db.bucket.RangeLimitIterator(start, stop, store.RangeROpen, 0, -1)
	defer it.Close()

	for ; it.Valid(); it.Next() {
		_, m, err := db.sDecodeSetKey(it.Key())
		if err != nil {
			return nil, err
		}

		v = append(v, m)
	}

	return v, nil
}

// SRem removes the members of set.
func (db *DB) SRem(key []byte, args ...[]byte) (int64, error) {
	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	var ek []byte
	var v []byte
	var err error

	it := db.bucket.NewIterator()
	defer it.Close()

	var num int64
	for i := 0; i < len(args); i++ {
		if err := checkSetKMSize(key, args[i]); err != nil {
			return 0, err
		}

		ek = db.sEncodeSetKey(key, args[i])

		v = it.RawFind(ek)
		if v == nil {
			continue
		} else {
			num++
			t.Delete(ek)
		}
	}

	if _, err = db.sIncrSize(key, -num); err != nil {
		return 0, err
	}

	err = t.Commit()
	return num, err

}

func (db *DB) sUnionGeneric(keys ...[]byte) ([][]byte, error) {
	dstMap := make(map[string]bool)

	for _, key := range keys {
		if err := checkKeySize(key); err != nil {
			return nil, err
		}

		members, err := db.SMembers(key)
		if err != nil {
			return nil, err
		}

		for _, member := range members {
			dstMap[hack.String(member)] = true
		}
	}

	slice := make([][]byte, len(dstMap))
	idx := 0
	for k, v := range dstMap {
		if !v {
			continue
		}
		slice[idx] = []byte(k)
		idx++
	}

	return slice, nil
}

// SUnion unions the sets.
func (db *DB) SUnion(keys ...[]byte) ([][]byte, error) {
	v, err := db.sUnionGeneric(keys...)
	return v, err
}

// SUnionStore unions the sets and stores to the dest set.
func (db *DB) SUnionStore(dstKey []byte, keys ...[]byte) (int64, error) {
	n, err := db.sStoreGeneric(dstKey, UnionType, keys...)
	return n, err
}

func (db *DB) sStoreGeneric(dstKey []byte, optType byte, keys ...[]byte) (int64, error) {
	if err := checkKeySize(dstKey); err != nil {
		return 0, err
	}

	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	db.sDelete(t, dstKey)

	var err error
	var ek []byte
	var v [][]byte

	switch optType {
	case UnionType:
		v, err = db.sUnionGeneric(keys...)
	case DiffType:
		v, err = db.sDiffGeneric(keys...)
	case InterType:
		v, err = db.sInterGeneric(keys...)
	}

	if err != nil {
		return 0, err
	}

	for _, m := range v {
		if err := checkSetKMSize(dstKey, m); err != nil {
			return 0, err
		}

		ek = db.sEncodeSetKey(dstKey, m)

		if _, err := db.bucket.Get(ek); err != nil {
			return 0, err
		}

		t.Put(ek, nil)
	}

	var n = int64(len(v))
	sk := db.sEncodeSizeKey(dstKey)
	t.Put(sk, PutInt64(n))

	if err = t.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// SClear clears the set.
func (db *DB) SClear(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	num := db.sDelete(t, key)
	db.rmExpire(t, SetType, key)

	err := t.Commit()
	return num, err
}

// SMclear clears multi sets.
func (db *DB) SMclear(keys ...[]byte) (int64, error) {
	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	for _, key := range keys {
		if err := checkKeySize(key); err != nil {
			return 0, err
		}

		db.sDelete(t, key)
		db.rmExpire(t, SetType, key)
	}

	err := t.Commit()
	return int64(len(keys)), err
}

// SExpire expries the set.
func (db *DB) SExpire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	return db.sExpireAt(key, time.Now().Unix()+duration)

}

// SExpireAt expires the set at when.
func (db *DB) SExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	return db.sExpireAt(key, when)

}

// STTL gets the TTL of set.
func (db *DB) STTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(SetType, key)
}

// SPersist removes the TTL of set.
func (db *DB) SPersist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.setBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.rmExpire(t, SetType, key)
	if err != nil {
		return 0, err
	}
	err = t.Commit()
	return n, err
}
