package ledis

import (
	"errors"
	"regexp"

	"github.com/siddontang/ledisdb/store"
)

var errDataType = errors.New("error data type")
var errMetaKey = errors.New("error meta key")

//Scan scans the data. If inclusive is true, scan range [cursor, inf) else (cursor, inf)
func (db *DB) Scan(dataType DataType, cursor []byte, count int, inclusive bool, match string) ([][]byte, error) {
	storeDataType, err := getDataStoreType(dataType)
	if err != nil {
		return nil, err
	}

	return db.scanGeneric(storeDataType, cursor, count, inclusive, match, false)
}

// RevScan scans the data reversed. if inclusive is true, revscan range (-inf, cursor] else (inf, cursor)
func (db *DB) RevScan(dataType DataType, cursor []byte, count int, inclusive bool, match string) ([][]byte, error) {
	storeDataType, err := getDataStoreType(dataType)
	if err != nil {
		return nil, err
	}

	return db.scanGeneric(storeDataType, cursor, count, inclusive, match, true)
}

func getDataStoreType(dataType DataType) (byte, error) {
	var storeDataType byte
	switch dataType {
	case KV:
		storeDataType = KVType
	case LIST:
		storeDataType = LMetaType
	case HASH:
		storeDataType = HSizeType
	case SET:
		storeDataType = SSizeType
	case ZSET:
		storeDataType = ZSizeType
	default:
		return 0, errDataType
	}
	return storeDataType, nil
}

func buildMatchRegexp(match string) (*regexp.Regexp, error) {
	var err error
	var r *regexp.Regexp

	if len(match) > 0 {
		if r, err = regexp.Compile(match); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (db *DB) buildScanIterator(minKey []byte, maxKey []byte, inclusive bool, reverse bool) *store.RangeLimitIterator {
	tp := store.RangeOpen

	if !reverse {
		if inclusive {
			tp = store.RangeROpen
		}
	} else {
		if inclusive {
			tp = store.RangeLOpen
		}
	}

	var it *store.RangeLimitIterator
	if !reverse {
		it = db.bucket.RangeIterator(minKey, maxKey, tp)
	} else {
		it = db.bucket.RevRangeIterator(minKey, maxKey, tp)
	}

	return it
}

func (db *DB) buildScanKeyRange(storeDataType byte, key []byte, reverse bool) (minKey []byte, maxKey []byte, err error) {
	if !reverse {
		if minKey, err = db.encodeScanMinKey(storeDataType, key); err != nil {
			return
		}
		if maxKey, err = db.encodeScanMaxKey(storeDataType, nil); err != nil {
			return
		}
	} else {
		if minKey, err = db.encodeScanMinKey(storeDataType, nil); err != nil {
			return
		}
		if maxKey, err = db.encodeScanMaxKey(storeDataType, key); err != nil {
			return
		}
	}
	return
}

func checkScanCount(count int) int {
	if count <= 0 {
		count = defaultScanCount
	}

	return count
}

func (db *DB) scanGeneric(storeDataType byte, key []byte, count int,
	inclusive bool, match string, reverse bool) ([][]byte, error) {

	r, err := buildMatchRegexp(match)
	if err != nil {
		return nil, err
	}

	minKey, maxKey, err := db.buildScanKeyRange(storeDataType, key, reverse)
	if err != nil {
		return nil, err
	}

	count = checkScanCount(count)

	it := db.buildScanIterator(minKey, maxKey, inclusive, reverse)

	v := make([][]byte, 0, count)

	for i := 0; it.Valid() && i < count; it.Next() {
		if k, err := db.decodeScanKey(storeDataType, it.Key()); err != nil {
			continue
		} else if r != nil && !r.Match(k) {
			continue
		} else {
			v = append(v, k)
			i++
		}
	}
	it.Close()
	return v, nil
}

func (db *DB) encodeScanMinKey(storeDataType byte, key []byte) ([]byte, error) {
	return db.encodeScanKey(storeDataType, key)
}

func (db *DB) encodeScanMaxKey(storeDataType byte, key []byte) ([]byte, error) {
	if len(key) > 0 {
		return db.encodeScanKey(storeDataType, key)
	}

	k, err := db.encodeScanKey(storeDataType, nil)
	if err != nil {
		return nil, err
	}
	k[len(k)-1] = storeDataType + 1
	return k, nil
}

func (db *DB) encodeScanKey(storeDataType byte, key []byte) ([]byte, error) {
	switch storeDataType {
	case KVType:
		return db.encodeKVKey(key), nil
	case LMetaType:
		return db.lEncodeMetaKey(key), nil
	case HSizeType:
		return db.hEncodeSizeKey(key), nil
	case ZSizeType:
		return db.zEncodeSizeKey(key), nil
	case SSizeType:
		return db.sEncodeSizeKey(key), nil
	default:
		return nil, errDataType
	}
}

func (db *DB) decodeScanKey(storeDataType byte, ek []byte) (key []byte, err error) {
	switch storeDataType {
	case KVType:
		key, err = db.decodeKVKey(ek)
	case LMetaType:
		key, err = db.lDecodeMetaKey(ek)
	case HSizeType:
		key, err = db.hDecodeSizeKey(ek)
	case ZSizeType:
		key, err = db.zDecodeSizeKey(ek)
	case SSizeType:
		key, err = db.sDecodeSizeKey(ek)
	default:
		err = errDataType
	}
	return
}

// for specail data scan

func (db *DB) buildDataScanKeyRange(storeDataType byte, key []byte, cursor []byte, reverse bool) (minKey []byte, maxKey []byte, err error) {
	if !reverse {
		if minKey, err = db.encodeDataScanMinKey(storeDataType, key, cursor); err != nil {
			return
		}
		if maxKey, err = db.encodeDataScanMaxKey(storeDataType, key, nil); err != nil {
			return
		}
	} else {
		if minKey, err = db.encodeDataScanMinKey(storeDataType, key, nil); err != nil {
			return
		}
		if maxKey, err = db.encodeDataScanMaxKey(storeDataType, key, cursor); err != nil {
			return
		}
	}
	return
}

func (db *DB) encodeDataScanMinKey(storeDataType byte, key []byte, cursor []byte) ([]byte, error) {
	return db.encodeDataScanKey(storeDataType, key, cursor)
}

func (db *DB) encodeDataScanMaxKey(storeDataType byte, key []byte, cursor []byte) ([]byte, error) {
	if len(cursor) > 0 {
		return db.encodeDataScanKey(storeDataType, key, cursor)
	}

	k, err := db.encodeDataScanKey(storeDataType, key, nil)
	if err != nil {
		return nil, err
	}

	// here, the last byte is the start seperator, set it to stop seperator
	k[len(k)-1] = k[len(k)-1] + 1
	return k, nil
}

func (db *DB) encodeDataScanKey(storeDataType byte, key []byte, cursor []byte) ([]byte, error) {
	switch storeDataType {
	case HashType:
		return db.hEncodeHashKey(key, cursor), nil
	case ZSetType:
		return db.zEncodeSetKey(key, cursor), nil
	case SetType:
		return db.sEncodeSetKey(key, cursor), nil
	default:
		return nil, errDataType
	}
}

func (db *DB) buildDataScanIterator(storeDataType byte, key []byte, cursor []byte, count int,
	inclusive bool, reverse bool) (*store.RangeLimitIterator, error) {

	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	minKey, maxKey, err := db.buildDataScanKeyRange(storeDataType, key, cursor, reverse)
	if err != nil {
		return nil, err
	}

	it := db.buildScanIterator(minKey, maxKey, inclusive, reverse)

	return it, nil
}

func (db *DB) hScanGeneric(key []byte, cursor []byte, count int, inclusive bool, match string, reverse bool) ([]FVPair, error) {
	count = checkScanCount(count)

	r, err := buildMatchRegexp(match)
	if err != nil {
		return nil, err
	}

	v := make([]FVPair, 0, count)

	it, err := db.buildDataScanIterator(HashType, key, cursor, count, inclusive, reverse)
	if err != nil {
		return nil, err
	}

	defer it.Close()

	for i := 0; it.Valid() && i < count; it.Next() {
		_, f, err := db.hDecodeHashKey(it.Key())
		if err != nil {
			return nil, err
		} else if r != nil && !r.Match(f) {
			continue
		}

		v = append(v, FVPair{Field: f, Value: it.Value()})

		i++
	}

	return v, nil
}

// HScan scans data for hash.
func (db *DB) HScan(key []byte, cursor []byte, count int, inclusive bool, match string) ([]FVPair, error) {
	return db.hScanGeneric(key, cursor, count, inclusive, match, false)
}

// HRevScan reversed scans data for hash.
func (db *DB) HRevScan(key []byte, cursor []byte, count int, inclusive bool, match string) ([]FVPair, error) {
	return db.hScanGeneric(key, cursor, count, inclusive, match, true)
}

func (db *DB) sScanGeneric(key []byte, cursor []byte, count int, inclusive bool, match string, reverse bool) ([][]byte, error) {
	count = checkScanCount(count)

	r, err := buildMatchRegexp(match)
	if err != nil {
		return nil, err
	}

	v := make([][]byte, 0, count)

	it, err := db.buildDataScanIterator(SetType, key, cursor, count, inclusive, reverse)
	if err != nil {
		return nil, err
	}

	defer it.Close()

	for i := 0; it.Valid() && i < count; it.Next() {
		_, m, err := db.sDecodeSetKey(it.Key())
		if err != nil {
			return nil, err
		} else if r != nil && !r.Match(m) {
			continue
		}

		v = append(v, m)

		i++
	}

	return v, nil
}

// SScan scans data for set.
func (db *DB) SScan(key []byte, cursor []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.sScanGeneric(key, cursor, count, inclusive, match, false)
}

// SRevScan scans data reversed for set.
func (db *DB) SRevScan(key []byte, cursor []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.sScanGeneric(key, cursor, count, inclusive, match, true)
}

func (db *DB) zScanGeneric(key []byte, cursor []byte, count int, inclusive bool, match string, reverse bool) ([]ScorePair, error) {
	count = checkScanCount(count)

	r, err := buildMatchRegexp(match)
	if err != nil {
		return nil, err
	}

	v := make([]ScorePair, 0, count)

	it, err := db.buildDataScanIterator(ZSetType, key, cursor, count, inclusive, reverse)
	if err != nil {
		return nil, err
	}

	defer it.Close()

	for i := 0; it.Valid() && i < count; it.Next() {
		_, m, err := db.zDecodeSetKey(it.Key())
		if err != nil {
			return nil, err
		} else if r != nil && !r.Match(m) {
			continue
		}

		score, err := Int64(it.Value(), nil)
		if err != nil {
			return nil, err
		}

		v = append(v, ScorePair{Score: score, Member: m})

		i++
	}

	return v, nil
}

// ZScan scans data for zset.
func (db *DB) ZScan(key []byte, cursor []byte, count int, inclusive bool, match string) ([]ScorePair, error) {
	return db.zScanGeneric(key, cursor, count, inclusive, match, false)
}

// ZRevScan scans data reversed for zset.
func (db *DB) ZRevScan(key []byte, cursor []byte, count int, inclusive bool, match string) ([]ScorePair, error) {
	return db.zScanGeneric(key, cursor, count, inclusive, match, true)
}
