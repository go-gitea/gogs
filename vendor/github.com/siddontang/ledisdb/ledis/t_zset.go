package ledis

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/siddontang/go/hack"
	"github.com/siddontang/ledisdb/store"
)

// For zset const.
const (
	MinScore     int64 = -1<<63 + 1
	MaxScore     int64 = 1<<63 - 1
	InvalidScore int64 = -1 << 63

	AggregateSum byte = 0
	AggregateMin byte = 1
	AggregateMax byte = 2
)

// ScorePair is the pair of score and member.
type ScorePair struct {
	Score  int64
	Member []byte
}

var errZSizeKey = errors.New("invalid zsize key")
var errZSetKey = errors.New("invalid zset key")
var errZScoreKey = errors.New("invalid zscore key")
var errScoreOverflow = errors.New("zset score overflow")
var errInvalidAggregate = errors.New("invalid aggregate")
var errInvalidWeightNum = errors.New("invalid weight number")
var errInvalidSrcKeyNum = errors.New("invalid src key number")

const (
	zsetNScoreSep    byte = '<'
	zsetPScoreSep    byte = zsetNScoreSep + 1
	zsetStopScoreSep byte = zsetPScoreSep + 1

	zsetStartMemSep byte = ':'
	zsetStopMemSep  byte = zsetStartMemSep + 1
)

func checkZSetKMSize(key []byte, member []byte) error {
	if len(key) > MaxKeySize || len(key) == 0 {
		return errKeySize
	} else if len(member) > MaxZSetMemberSize || len(member) == 0 {
		return errZSetMemberSize
	}
	return nil
}

func (db *DB) zEncodeSizeKey(key []byte) []byte {
	buf := make([]byte, len(key)+1+len(db.indexVarBuf))
	pos := copy(buf, db.indexVarBuf)
	buf[pos] = ZSizeType
	pos++
	copy(buf[pos:], key)
	return buf
}

func (db *DB) zDecodeSizeKey(ek []byte) ([]byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, err
	}

	if pos+1 > len(ek) || ek[pos] != ZSizeType {
		return nil, errZSizeKey
	}
	pos++
	return ek[pos:], nil
}

func (db *DB) zEncodeSetKey(key []byte, member []byte) []byte {
	buf := make([]byte, len(key)+len(member)+4+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)

	buf[pos] = ZSetType
	pos++

	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2

	copy(buf[pos:], key)
	pos += len(key)

	buf[pos] = zsetStartMemSep
	pos++

	copy(buf[pos:], member)

	return buf
}

func (db *DB) zDecodeSetKey(ek []byte) ([]byte, []byte, error) {
	pos, err := db.checkKeyIndex(ek)
	if err != nil {
		return nil, nil, err
	}

	if pos+1 > len(ek) || ek[pos] != ZSetType {
		return nil, nil, errZSetKey
	}

	pos++

	if pos+2 > len(ek) {
		return nil, nil, errZSetKey
	}

	keyLen := int(binary.BigEndian.Uint16(ek[pos:]))
	if keyLen+pos > len(ek) {
		return nil, nil, errZSetKey
	}

	pos += 2
	key := ek[pos : pos+keyLen]

	if ek[pos+keyLen] != zsetStartMemSep {
		return nil, nil, errZSetKey
	}
	pos++

	member := ek[pos+keyLen:]
	return key, member, nil
}

func (db *DB) zEncodeStartSetKey(key []byte) []byte {
	k := db.zEncodeSetKey(key, nil)
	return k
}

func (db *DB) zEncodeStopSetKey(key []byte) []byte {
	k := db.zEncodeSetKey(key, nil)
	k[len(k)-1] = zsetStartMemSep + 1
	return k
}

func (db *DB) zEncodeScoreKey(key []byte, member []byte, score int64) []byte {
	buf := make([]byte, len(key)+len(member)+13+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)

	buf[pos] = ZScoreType
	pos++

	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2

	copy(buf[pos:], key)
	pos += len(key)

	if score < 0 {
		buf[pos] = zsetNScoreSep
	} else {
		buf[pos] = zsetPScoreSep
	}

	pos++
	binary.BigEndian.PutUint64(buf[pos:], uint64(score))
	pos += 8

	buf[pos] = zsetStartMemSep
	pos++

	copy(buf[pos:], member)
	return buf
}

func (db *DB) zEncodeStartScoreKey(key []byte, score int64) []byte {
	return db.zEncodeScoreKey(key, nil, score)
}

func (db *DB) zEncodeStopScoreKey(key []byte, score int64) []byte {
	k := db.zEncodeScoreKey(key, nil, score)
	k[len(k)-1] = zsetStopMemSep
	return k
}

func (db *DB) zDecodeScoreKey(ek []byte) (key []byte, member []byte, score int64, err error) {
	pos := 0
	pos, err = db.checkKeyIndex(ek)
	if err != nil {
		return
	}

	if pos+1 > len(ek) || ek[pos] != ZScoreType {
		err = errZScoreKey
		return
	}
	pos++

	if pos+2 > len(ek) {
		err = errZScoreKey
		return
	}
	keyLen := int(binary.BigEndian.Uint16(ek[pos:]))
	pos += 2

	if keyLen+pos > len(ek) {
		err = errZScoreKey
		return
	}

	key = ek[pos : pos+keyLen]
	pos += keyLen

	if pos+10 > len(ek) {
		err = errZScoreKey
		return
	}

	if (ek[pos] != zsetNScoreSep) && (ek[pos] != zsetPScoreSep) {
		err = errZScoreKey
		return
	}
	pos++

	score = int64(binary.BigEndian.Uint64(ek[pos:]))
	pos += 8

	if ek[pos] != zsetStartMemSep {
		err = errZScoreKey
		return
	}

	pos++

	member = ek[pos:]
	return
}

func (db *DB) zSetItem(t *batch, key []byte, score int64, member []byte) (int64, error) {
	if score <= MinScore || score >= MaxScore {
		return 0, errScoreOverflow
	}

	var exists int64
	ek := db.zEncodeSetKey(key, member)

	if v, err := db.bucket.Get(ek); err != nil {
		return 0, err
	} else if v != nil {
		exists = 1

		s, err := Int64(v, err)
		if err != nil {
			return 0, err
		}

		sk := db.zEncodeScoreKey(key, member, s)
		t.Delete(sk)
	}

	t.Put(ek, PutInt64(score))

	sk := db.zEncodeScoreKey(key, member, score)
	t.Put(sk, []byte{})

	return exists, nil
}

func (db *DB) zDelItem(t *batch, key []byte, member []byte, skipDelScore bool) (int64, error) {
	ek := db.zEncodeSetKey(key, member)
	if v, err := db.bucket.Get(ek); err != nil {
		return 0, err
	} else if v == nil {
		//not exists
		return 0, nil
	} else {
		//exists
		if !skipDelScore {
			//we must del score
			s, err := Int64(v, err)
			if err != nil {
				return 0, err
			}
			sk := db.zEncodeScoreKey(key, member, s)
			t.Delete(sk)
		}
	}

	t.Delete(ek)

	return 1, nil
}

func (db *DB) zDelete(t *batch, key []byte) int64 {
	delMembCnt, _ := db.zRemRange(t, key, MinScore, MaxScore, 0, -1)
	//	todo : log err
	return delMembCnt
}

func (db *DB) zExpireAt(key []byte, when int64) (int64, error) {
	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	if zcnt, err := db.ZCard(key); err != nil || zcnt == 0 {
		return 0, err
	}

	db.expireAt(t, ZSetType, key, when)
	if err := t.Commit(); err != nil {
		return 0, err
	}

	return 1, nil
}

// ZAdd add the members.
func (db *DB) ZAdd(key []byte, args ...ScorePair) (int64, error) {
	if len(args) == 0 {
		return 0, nil
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	var num int64
	for i := 0; i < len(args); i++ {
		score := args[i].Score
		member := args[i].Member

		if err := checkZSetKMSize(key, member); err != nil {
			return 0, err
		}

		if n, err := db.zSetItem(t, key, score, member); err != nil {
			return 0, err
		} else if n == 0 {
			//add new
			num++
		}
	}

	if _, err := db.zIncrSize(t, key, num); err != nil {
		return 0, err
	}

	err := t.Commit()
	return num, err
}

func (db *DB) zIncrSize(t *batch, key []byte, delta int64) (int64, error) {
	sk := db.zEncodeSizeKey(key)

	size, err := Int64(db.bucket.Get(sk))
	if err != nil {
		return 0, err
	}
	size += delta
	if size <= 0 {
		size = 0
		t.Delete(sk)
		db.rmExpire(t, ZSetType, key)
	} else {
		t.Put(sk, PutInt64(size))
	}

	return size, nil
}

// ZCard gets the size of the zset.
func (db *DB) ZCard(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	sk := db.zEncodeSizeKey(key)
	return Int64(db.bucket.Get(sk))
}

// ZScore gets the score of member.
func (db *DB) ZScore(key []byte, member []byte) (int64, error) {
	if err := checkZSetKMSize(key, member); err != nil {
		return InvalidScore, err
	}

	score := InvalidScore

	k := db.zEncodeSetKey(key, member)
	if v, err := db.bucket.Get(k); err != nil {
		return InvalidScore, err
	} else if v == nil {
		return InvalidScore, ErrScoreMiss
	} else {
		if score, err = Int64(v, nil); err != nil {
			return InvalidScore, err
		}
	}

	return score, nil
}

// ZRem removes members
func (db *DB) ZRem(key []byte, members ...[]byte) (int64, error) {
	if len(members) == 0 {
		return 0, nil
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	var num int64
	for i := 0; i < len(members); i++ {
		if err := checkZSetKMSize(key, members[i]); err != nil {
			return 0, err
		}

		if n, err := db.zDelItem(t, key, members[i], false); err != nil {
			return 0, err
		} else if n == 1 {
			num++
		}
	}

	if _, err := db.zIncrSize(t, key, -num); err != nil {
		return 0, err
	}

	err := t.Commit()
	return num, err
}

// ZIncrBy increases the score of member with delta.
func (db *DB) ZIncrBy(key []byte, delta int64, member []byte) (int64, error) {
	if err := checkZSetKMSize(key, member); err != nil {
		return InvalidScore, err
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	ek := db.zEncodeSetKey(key, member)

	var oldScore int64
	v, err := db.bucket.Get(ek)
	if err != nil {
		return InvalidScore, err
	} else if v == nil {
		db.zIncrSize(t, key, 1)
	} else {
		if oldScore, err = Int64(v, err); err != nil {
			return InvalidScore, err
		}
	}

	newScore := oldScore + delta
	if newScore >= MaxScore || newScore <= MinScore {
		return InvalidScore, errScoreOverflow
	}

	sk := db.zEncodeScoreKey(key, member, newScore)
	t.Put(sk, []byte{})
	t.Put(ek, PutInt64(newScore))

	if v != nil {
		// so as to update score, we must delete the old one
		oldSk := db.zEncodeScoreKey(key, member, oldScore)
		t.Delete(oldSk)
	}

	err = t.Commit()
	return newScore, err
}

// ZCount gets the number of score in [min, max]
func (db *DB) ZCount(key []byte, min int64, max int64) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}
	minKey := db.zEncodeStartScoreKey(key, min)
	maxKey := db.zEncodeStopScoreKey(key, max)

	rangeType := store.RangeROpen

	it := db.bucket.RangeLimitIterator(minKey, maxKey, rangeType, 0, -1)
	var n int64
	for ; it.Valid(); it.Next() {
		n++
	}
	it.Close()

	return n, nil
}

func (db *DB) zrank(key []byte, member []byte, reverse bool) (int64, error) {
	if err := checkZSetKMSize(key, member); err != nil {
		return 0, err
	}

	k := db.zEncodeSetKey(key, member)

	it := db.bucket.NewIterator()
	defer it.Close()

	v := it.Find(k)
	if v == nil {
		return -1, nil
	}

	s, err := Int64(v, nil)
	if err != nil {
		return 0, err
	}
	var rit *store.RangeLimitIterator

	sk := db.zEncodeScoreKey(key, member, s)

	if !reverse {
		minKey := db.zEncodeStartScoreKey(key, MinScore)

		rit = store.NewRangeIterator(it, &store.Range{Min: minKey, Max: sk, Type: store.RangeClose})
	} else {
		maxKey := db.zEncodeStopScoreKey(key, MaxScore)
		rit = store.NewRevRangeIterator(it, &store.Range{Min: sk, Max: maxKey, Type: store.RangeClose})
	}

	var lastKey []byte
	var n int64

	for ; rit.Valid(); rit.Next() {
		n++

		lastKey = rit.BufKey(lastKey)
	}

	if _, m, _, err := db.zDecodeScoreKey(lastKey); err == nil && bytes.Equal(m, member) {
		n--
		return n, nil
	}

	return -1, nil
}

func (db *DB) zIterator(key []byte, min int64, max int64, offset int, count int, reverse bool) *store.RangeLimitIterator {
	minKey := db.zEncodeStartScoreKey(key, min)
	maxKey := db.zEncodeStopScoreKey(key, max)

	if !reverse {
		return db.bucket.RangeLimitIterator(minKey, maxKey, store.RangeClose, offset, count)
	}
	return db.bucket.RevRangeLimitIterator(minKey, maxKey, store.RangeClose, offset, count)
}

func (db *DB) zRemRange(t *batch, key []byte, min int64, max int64, offset int, count int) (int64, error) {
	if len(key) > MaxKeySize {
		return 0, errKeySize
	}

	it := db.zIterator(key, min, max, offset, count, false)
	var num int64
	for ; it.Valid(); it.Next() {
		sk := it.RawKey()
		_, m, _, err := db.zDecodeScoreKey(sk)
		if err != nil {
			continue
		}

		if n, err := db.zDelItem(t, key, m, true); err != nil {
			return 0, err
		} else if n == 1 {
			num++
		}

		t.Delete(sk)
	}
	it.Close()

	if _, err := db.zIncrSize(t, key, -num); err != nil {
		return 0, err
	}

	return num, nil
}

func (db *DB) zRange(key []byte, min int64, max int64, offset int, count int, reverse bool) ([]ScorePair, error) {
	if len(key) > MaxKeySize {
		return nil, errKeySize
	}

	if offset < 0 {
		return []ScorePair{}, nil
	}

	nv := count
	// count may be very large, so we must limit it for below mem make.
	if nv <= 0 || nv > 1024 {
		nv = 64
	}

	v := make([]ScorePair, 0, nv)

	var it *store.RangeLimitIterator

	//if reverse and offset is 0, count < 0, we may use forward iterator then reverse
	//because store iterator prev is slower than next
	if !reverse || (offset == 0 && count < 0) {
		it = db.zIterator(key, min, max, offset, count, false)
	} else {
		it = db.zIterator(key, min, max, offset, count, true)
	}

	for ; it.Valid(); it.Next() {
		_, m, s, err := db.zDecodeScoreKey(it.Key())
		//may be we will check key equal?
		if err != nil {
			continue
		}

		v = append(v, ScorePair{Member: m, Score: s})
	}
	it.Close()

	if reverse && (offset == 0 && count < 0) {
		for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
			v[i], v[j] = v[j], v[i]
		}
	}

	return v, nil
}

func (db *DB) zParseLimit(key []byte, start int, stop int) (offset int, count int, err error) {
	if start < 0 || stop < 0 {
		//refer redis implementation
		var size int64
		size, err = db.ZCard(key)
		if err != nil {
			return
		}

		llen := int(size)

		if start < 0 {
			start = llen + start
		}
		if stop < 0 {
			stop = llen + stop
		}

		if start < 0 {
			start = 0
		}

		if start >= llen {
			offset = -1
			return
		}
	}

	if start > stop {
		offset = -1
		return
	}

	offset = start
	count = (stop - start) + 1
	return
}

// ZClear clears the zset.
func (db *DB) ZClear(key []byte) (int64, error) {
	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	rmCnt, err := db.zRemRange(t, key, MinScore, MaxScore, 0, -1)
	if err == nil {
		err = t.Commit()
	}

	return rmCnt, err
}

// ZMclear clears multi zsets.
func (db *DB) ZMclear(keys ...[]byte) (int64, error) {
	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	for _, key := range keys {
		if _, err := db.zRemRange(t, key, MinScore, MaxScore, 0, -1); err != nil {
			return 0, err
		}
	}

	err := t.Commit()

	return int64(len(keys)), err
}

// ZRange gets the members from start to stop.
func (db *DB) ZRange(key []byte, start int, stop int) ([]ScorePair, error) {
	return db.ZRangeGeneric(key, start, stop, false)
}

// ZRangeByScore gets the data with score in min and max.
// min and max must be inclusive
// if no limit, set offset = 0 and count = -1
func (db *DB) ZRangeByScore(key []byte, min int64, max int64,
	offset int, count int) ([]ScorePair, error) {
	return db.ZRangeByScoreGeneric(key, min, max, offset, count, false)
}

// ZRank gets the rank of member.
func (db *DB) ZRank(key []byte, member []byte) (int64, error) {
	return db.zrank(key, member, false)
}

// ZRemRangeByRank removes the member at range from start to stop.
func (db *DB) ZRemRangeByRank(key []byte, start int, stop int) (int64, error) {
	offset, count, err := db.zParseLimit(key, start, stop)
	if err != nil {
		return 0, err
	}

	var rmCnt int64

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	rmCnt, err = db.zRemRange(t, key, MinScore, MaxScore, offset, count)
	if err == nil {
		err = t.Commit()
	}

	return rmCnt, err
}

// ZRemRangeByScore removes the data with score at [min, max]
func (db *DB) ZRemRangeByScore(key []byte, min int64, max int64) (int64, error) {
	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	rmCnt, err := db.zRemRange(t, key, min, max, 0, -1)
	if err == nil {
		err = t.Commit()
	}

	return rmCnt, err
}

// ZRevRange gets the data reversed.
func (db *DB) ZRevRange(key []byte, start int, stop int) ([]ScorePair, error) {
	return db.ZRangeGeneric(key, start, stop, true)
}

// ZRevRank gets the rank of member reversed.
func (db *DB) ZRevRank(key []byte, member []byte) (int64, error) {
	return db.zrank(key, member, true)
}

// ZRevRangeByScore gets the data with score at [min, max]
// min and max must be inclusive
// if no limit, set offset = 0 and count = -1
func (db *DB) ZRevRangeByScore(key []byte, min int64, max int64, offset int, count int) ([]ScorePair, error) {
	return db.ZRangeByScoreGeneric(key, min, max, offset, count, true)
}

// ZRangeGeneric is a generic function for scan zset.
func (db *DB) ZRangeGeneric(key []byte, start int, stop int, reverse bool) ([]ScorePair, error) {
	offset, count, err := db.zParseLimit(key, start, stop)
	if err != nil {
		return nil, err
	}

	return db.zRange(key, MinScore, MaxScore, offset, count, reverse)
}

// ZRangeByScoreGeneric is a generic function to scan zset with score.
// min and max must be inclusive
// if no limit, set offset = 0 and count = -1
func (db *DB) ZRangeByScoreGeneric(key []byte, min int64, max int64,
	offset int, count int, reverse bool) ([]ScorePair, error) {

	return db.zRange(key, min, max, offset, count, reverse)
}

func (db *DB) zFlush() (drop int64, err error) {
	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()
	return db.flushType(t, ZSetType)
}

// ZExpire expires the zset.
func (db *DB) ZExpire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	return db.zExpireAt(key, time.Now().Unix()+duration)
}

// ZExpireAt expires the zset at when.
func (db *DB) ZExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	return db.zExpireAt(key, when)
}

// ZTTL gets the TTL of zset.
func (db *DB) ZTTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(ZSetType, key)
}

// ZPersist removes the TTL of zset.
func (db *DB) ZPersist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.rmExpire(t, ZSetType, key)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

func getAggregateFunc(aggregate byte) func(int64, int64) int64 {
	switch aggregate {
	case AggregateSum:
		return func(a int64, b int64) int64 {
			return a + b
		}
	case AggregateMax:
		return func(a int64, b int64) int64 {
			if a > b {
				return a
			}
			return b
		}
	case AggregateMin:
		return func(a int64, b int64) int64 {
			if a > b {
				return b
			}
			return a
		}
	}
	return nil
}

// ZUnionStore unions the zsets and stores to dest zset.
func (db *DB) ZUnionStore(destKey []byte, srcKeys [][]byte, weights []int64, aggregate byte) (int64, error) {

	var destMap = map[string]int64{}
	aggregateFunc := getAggregateFunc(aggregate)
	if aggregateFunc == nil {
		return 0, errInvalidAggregate
	}
	if len(srcKeys) < 1 {
		return 0, errInvalidSrcKeyNum
	}
	if weights != nil {
		if len(srcKeys) != len(weights) {
			return 0, errInvalidWeightNum
		}
	} else {
		weights = make([]int64, len(srcKeys))
		for i := 0; i < len(weights); i++ {
			weights[i] = 1
		}
	}

	for i, key := range srcKeys {
		scorePairs, err := db.ZRange(key, 0, -1)
		if err != nil {
			return 0, err
		}
		for _, pair := range scorePairs {
			if score, ok := destMap[hack.String(pair.Member)]; !ok {
				destMap[hack.String(pair.Member)] = pair.Score * weights[i]
			} else {
				destMap[hack.String(pair.Member)] = aggregateFunc(score, pair.Score*weights[i])
			}
		}
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	db.zDelete(t, destKey)

	for member, score := range destMap {
		if err := checkZSetKMSize(destKey, []byte(member)); err != nil {
			return 0, err
		}

		if _, err := db.zSetItem(t, destKey, score, []byte(member)); err != nil {
			return 0, err
		}
	}

	var n = int64(len(destMap))
	sk := db.zEncodeSizeKey(destKey)
	t.Put(sk, PutInt64(n))

	if err := t.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// ZInterStore intersects the zsets and stores to dest zset.
func (db *DB) ZInterStore(destKey []byte, srcKeys [][]byte, weights []int64, aggregate byte) (int64, error) {

	aggregateFunc := getAggregateFunc(aggregate)
	if aggregateFunc == nil {
		return 0, errInvalidAggregate
	}
	if len(srcKeys) < 1 {
		return 0, errInvalidSrcKeyNum
	}
	if weights != nil {
		if len(srcKeys) != len(weights) {
			return 0, errInvalidWeightNum
		}
	} else {
		weights = make([]int64, len(srcKeys))
		for i := 0; i < len(weights); i++ {
			weights[i] = 1
		}
	}

	var destMap = map[string]int64{}
	scorePairs, err := db.ZRange(srcKeys[0], 0, -1)
	if err != nil {
		return 0, err
	}
	for _, pair := range scorePairs {
		destMap[hack.String(pair.Member)] = pair.Score * weights[0]
	}

	for i, key := range srcKeys[1:] {
		scorePairs, err := db.ZRange(key, 0, -1)
		if err != nil {
			return 0, err
		}
		tmpMap := map[string]int64{}
		for _, pair := range scorePairs {
			if score, ok := destMap[hack.String(pair.Member)]; ok {
				tmpMap[hack.String(pair.Member)] = aggregateFunc(score, pair.Score*weights[i+1])
			}
		}
		destMap = tmpMap
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	db.zDelete(t, destKey)

	for member, score := range destMap {
		if err := checkZSetKMSize(destKey, []byte(member)); err != nil {
			return 0, err
		}
		if _, err := db.zSetItem(t, destKey, score, []byte(member)); err != nil {
			return 0, err
		}
	}

	n := int64(len(destMap))
	sk := db.zEncodeSizeKey(destKey)
	t.Put(sk, PutInt64(n))

	if err := t.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// ZRangeByLex scans the zset lexicographically
func (db *DB) ZRangeByLex(key []byte, min []byte, max []byte, rangeType uint8, offset int, count int) ([][]byte, error) {
	if min == nil {
		min = db.zEncodeStartSetKey(key)
	} else {
		min = db.zEncodeSetKey(key, min)
	}
	if max == nil {
		max = db.zEncodeStopSetKey(key)
	} else {
		max = db.zEncodeSetKey(key, max)
	}

	it := db.bucket.RangeLimitIterator(min, max, rangeType, offset, count)
	defer it.Close()

	ay := make([][]byte, 0, 16)
	for ; it.Valid(); it.Next() {
		if _, m, err := db.zDecodeSetKey(it.Key()); err == nil {
			ay = append(ay, m)
		}
	}

	return ay, nil
}

// ZRemRangeByLex remvoes members in [min, max] lexicographically
func (db *DB) ZRemRangeByLex(key []byte, min []byte, max []byte, rangeType uint8) (int64, error) {
	if min == nil {
		min = db.zEncodeStartSetKey(key)
	} else {
		min = db.zEncodeSetKey(key, min)
	}
	if max == nil {
		max = db.zEncodeStopSetKey(key)
	} else {
		max = db.zEncodeSetKey(key, max)
	}

	t := db.zsetBatch
	t.Lock()
	defer t.Unlock()

	it := db.bucket.RangeIterator(min, max, rangeType)
	defer it.Close()

	var n int64
	for ; it.Valid(); it.Next() {
		t.Delete(it.RawKey())
		n++
	}

	if err := t.Commit(); err != nil {
		return 0, err
	}

	return n, nil
}

// ZLexCount gets the count of zset lexicographically.
func (db *DB) ZLexCount(key []byte, min []byte, max []byte, rangeType uint8) (int64, error) {
	if min == nil {
		min = db.zEncodeStartSetKey(key)
	} else {
		min = db.zEncodeSetKey(key, min)
	}
	if max == nil {
		max = db.zEncodeStopSetKey(key)
	} else {
		max = db.zEncodeSetKey(key, max)
	}

	it := db.bucket.RangeIterator(min, max, rangeType)
	defer it.Close()

	var n int64
	for ; it.Valid(); it.Next() {
		n++
	}

	return n, nil
}

// ZKeyExists checks zset existed or not.
func (db *DB) ZKeyExists(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}
	sk := db.zEncodeSizeKey(key)
	v, err := db.bucket.Get(sk)
	if v != nil && err == nil {
		return 1, nil
	}
	return 0, err
}
