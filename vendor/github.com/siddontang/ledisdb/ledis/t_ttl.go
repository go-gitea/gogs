package ledis

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/siddontang/ledisdb/store"
)

var (
	errExpMetaKey = errors.New("invalid expire meta key")
	errExpTimeKey = errors.New("invalid expire time key")
)

type onExpired func(*batch, []byte) int64

type ttlChecker struct {
	sync.Mutex
	db  *DB
	txs []*batch
	cbs []onExpired

	//next check time
	nc int64
}

var errExpType = errors.New("invalid expire type")

func (db *DB) expEncodeTimeKey(dataType byte, key []byte, when int64) []byte {
	buf := make([]byte, len(key)+10+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)

	buf[pos] = ExpTimeType
	pos++

	binary.BigEndian.PutUint64(buf[pos:], uint64(when))
	pos += 8

	buf[pos] = dataType
	pos++

	copy(buf[pos:], key)

	return buf
}

func (db *DB) expEncodeMetaKey(dataType byte, key []byte) []byte {
	buf := make([]byte, len(key)+2+len(db.indexVarBuf))

	pos := copy(buf, db.indexVarBuf)
	buf[pos] = ExpMetaType
	pos++
	buf[pos] = dataType
	pos++

	copy(buf[pos:], key)

	return buf
}

func (db *DB) expDecodeMetaKey(mk []byte) (byte, []byte, error) {
	pos, err := db.checkKeyIndex(mk)
	if err != nil {
		return 0, nil, err
	}

	if pos+2 > len(mk) || mk[pos] != ExpMetaType {
		return 0, nil, errExpMetaKey
	}

	return mk[pos+1], mk[pos+2:], nil
}

func (db *DB) expDecodeTimeKey(tk []byte) (byte, []byte, int64, error) {
	pos, err := db.checkKeyIndex(tk)
	if err != nil {
		return 0, nil, 0, err
	}

	if pos+10 > len(tk) || tk[pos] != ExpTimeType {
		return 0, nil, 0, errExpTimeKey
	}

	return tk[pos+9], tk[pos+10:], int64(binary.BigEndian.Uint64(tk[pos+1:])), nil
}

func (db *DB) expire(t *batch, dataType byte, key []byte, duration int64) {
	db.expireAt(t, dataType, key, time.Now().Unix()+duration)
}

func (db *DB) expireAt(t *batch, dataType byte, key []byte, when int64) {
	mk := db.expEncodeMetaKey(dataType, key)
	tk := db.expEncodeTimeKey(dataType, key, when)

	t.Put(tk, mk)
	t.Put(mk, PutInt64(when))

	db.ttlChecker.setNextCheckTime(when, false)
}

func (db *DB) ttl(dataType byte, key []byte) (t int64, err error) {
	mk := db.expEncodeMetaKey(dataType, key)

	if t, err = Int64(db.bucket.Get(mk)); err != nil || t == 0 {
		t = -1
	} else {
		t -= time.Now().Unix()
		if t <= 0 {
			t = -1
		}
		// if t == -1 : to remove ????
	}

	return t, err
}

func (db *DB) rmExpire(t *batch, dataType byte, key []byte) (int64, error) {
	mk := db.expEncodeMetaKey(dataType, key)
	v, err := db.bucket.Get(mk)
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	}

	when, err2 := Int64(v, nil)
	if err2 != nil {
		return 0, err2
	}

	tk := db.expEncodeTimeKey(dataType, key, when)
	t.Delete(mk)
	t.Delete(tk)
	return 1, nil
}

func (c *ttlChecker) register(dataType byte, t *batch, f onExpired) {
	c.txs[dataType] = t
	c.cbs[dataType] = f
}

func (c *ttlChecker) setNextCheckTime(when int64, force bool) {
	c.Lock()
	if force {
		c.nc = when
	} else if c.nc > when {
		c.nc = when
	}
	c.Unlock()
}

func (c *ttlChecker) check() {
	now := time.Now().Unix()

	c.Lock()
	nc := c.nc
	c.Unlock()

	if now < nc {
		return
	}

	nc = now + 3600

	db := c.db
	dbGet := db.bucket.Get

	minKey := db.expEncodeTimeKey(NoneType, nil, 0)
	maxKey := db.expEncodeTimeKey(maxDataType, nil, nc)

	it := db.bucket.RangeLimitIterator(minKey, maxKey, store.RangeROpen, 0, -1)
	for ; it.Valid(); it.Next() {
		tk := it.RawKey()
		mk := it.RawValue()

		dt, k, nt, err := db.expDecodeTimeKey(tk)
		if err != nil {
			continue
		}

		if nt > now {
			//the next ttl check time is nt!
			nc = nt
			break
		}

		t := c.txs[dt]
		cb := c.cbs[dt]
		if tk == nil || cb == nil {
			continue
		}

		t.Lock()

		if exp, err := Int64(dbGet(mk)); err == nil {
			// check expire again
			if exp <= now {
				cb(t, k)
				t.Delete(tk)
				t.Delete(mk)

				t.Commit()
			}

		}

		t.Unlock()
	}
	it.Close()

	c.setNextCheckTime(nc, true)

	return
}
