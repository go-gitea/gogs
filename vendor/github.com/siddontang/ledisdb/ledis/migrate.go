package ledis

import (
	"fmt"

	"github.com/siddontang/rdb"
)

/*
   To support redis <-> ledisdb, the dump value format is the same as redis.
   We will not support bitmap, and may add bit operations for kv later.

   But you must know that we use int64 for zset score, not double.
   Only support rdb version 6.
*/

// Dump dumps the KV value of key
func (db *DB) Dump(key []byte) ([]byte, error) {
	v, err := db.Get(key)
	if err != nil {
		return nil, err
	} else if v == nil {
		return nil, err
	}

	return rdb.Dump(rdb.String(v))
}

// LDump dumps the list value of key
func (db *DB) LDump(key []byte) ([]byte, error) {
	v, err := db.LRange(key, 0, -1)
	if err != nil {
		return nil, err
	} else if len(v) == 0 {
		return nil, err
	}

	return rdb.Dump(rdb.List(v))
}

// HDump dumps the hash value of key
func (db *DB) HDump(key []byte) ([]byte, error) {
	v, err := db.HGetAll(key)
	if err != nil {
		return nil, err
	} else if len(v) == 0 {
		return nil, err
	}

	o := make(rdb.Hash, len(v))
	for i := 0; i < len(v); i++ {
		o[i].Field = v[i].Field
		o[i].Value = v[i].Value
	}

	return rdb.Dump(o)
}

// SDump dumps the set value of key
func (db *DB) SDump(key []byte) ([]byte, error) {
	v, err := db.SMembers(key)
	if err != nil {
		return nil, err
	} else if len(v) == 0 {
		return nil, err
	}

	return rdb.Dump(rdb.Set(v))
}

// ZDump dumps the zset value of key
func (db *DB) ZDump(key []byte) ([]byte, error) {
	v, err := db.ZRangeByScore(key, MinScore, MaxScore, 0, -1)
	if err != nil {
		return nil, err
	} else if len(v) == 0 {
		return nil, err
	}

	o := make(rdb.ZSet, len(v))
	for i := 0; i < len(v); i++ {
		o[i].Member = v[i].Member
		o[i].Score = float64(v[i].Score)
	}

	return rdb.Dump(o)
}

// Restore restores a key into database.
func (db *DB) Restore(key []byte, ttl int64, data []byte) error {
	d, err := rdb.DecodeDump(data)
	if err != nil {
		return err
	}

	//ttl is milliseconds, but we only support seconds
	//later may support milliseconds
	if ttl > 0 {
		ttl = ttl / 1e3
		if ttl == 0 {
			ttl = 1
		}
	}

	switch value := d.(type) {
	case rdb.String:
		if _, err = db.Del(key); err != nil {
			return err
		}

		if err = db.Set(key, value); err != nil {
			return err
		}

		if ttl > 0 {
			if _, err = db.Expire(key, ttl); err != nil {
				return err
			}
		}
	case rdb.Hash:
		//first clear old key
		if _, err = db.HClear(key); err != nil {
			return err
		}

		fv := make([]FVPair, len(value))
		for i := 0; i < len(value); i++ {
			fv[i] = FVPair{Field: value[i].Field, Value: value[i].Value}
		}

		if err = db.HMset(key, fv...); err != nil {
			return err
		}

		if ttl > 0 {
			if _, err = db.HExpire(key, ttl); err != nil {
				return err
			}
		}
	case rdb.List:
		//first clear old key
		if _, err = db.LClear(key); err != nil {
			return err
		}

		if _, err = db.RPush(key, value...); err != nil {
			return err
		}

		if ttl > 0 {
			if _, err = db.LExpire(key, ttl); err != nil {
				return err
			}
		}
	case rdb.ZSet:
		//first clear old key
		if _, err = db.ZClear(key); err != nil {
			return err
		}

		sp := make([]ScorePair, len(value))
		for i := 0; i < len(value); i++ {
			sp[i] = ScorePair{int64(value[i].Score), value[i].Member}
		}

		if _, err = db.ZAdd(key, sp...); err != nil {
			return err
		}

		if ttl > 0 {
			if _, err = db.ZExpire(key, ttl); err != nil {
				return err
			}
		}
	case rdb.Set:
		//first clear old key
		if _, err = db.SClear(key); err != nil {
			return err
		}

		if _, err = db.SAdd(key, value...); err != nil {
			return err
		}

		if ttl > 0 {
			if _, err = db.SExpire(key, ttl); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid data type %T", d)
	}

	return nil
}
