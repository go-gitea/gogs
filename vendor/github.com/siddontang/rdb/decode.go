package rdb

// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

import "fmt"

import (
	"github.com/cupcake/rdb"
	"github.com/cupcake/rdb/nopdecoder"
)

func DecodeDump(p []byte) (interface{}, error) {
	d := &decoder{}
	if err := rdb.DecodeDump(p, 0, nil, 0, d); err != nil {
		return nil, err
	}
	return d.obj, d.err
}

type decoder struct {
	nopdecoder.NopDecoder
	obj interface{}
	err error
}

func (d *decoder) initObject(obj interface{}) {
	if d.err != nil {
		return
	}
	if d.obj != nil {
		d.err = fmt.Errorf("invalid object, init again")
	} else {
		d.obj = obj
	}
}

func (d *decoder) Set(key, value []byte, expiry int64) {
	d.initObject(String(value))
}

func (d *decoder) StartHash(key []byte, length, expiry int64) {
	d.initObject(Hash(nil))
}

func (d *decoder) Hset(key, field, value []byte) {
	if d.err != nil {
		return
	}
	switch h := d.obj.(type) {
	default:
		d.err = fmt.Errorf("invalid object, not a hashmap")
	case Hash:
		v := struct {
			Field, Value []byte
		}{
			field,
			value,
		}
		d.obj = append(h, v)
	}
}

func (d *decoder) StartSet(key []byte, cardinality, expiry int64) {
	d.initObject(Set(nil))
}

func (d *decoder) Sadd(key, member []byte) {
	if d.err != nil {
		return
	}
	switch s := d.obj.(type) {
	default:
		d.err = fmt.Errorf("invalid object, not a set")
	case Set:
		d.obj = append(s, member)
	}
}

func (d *decoder) StartList(key []byte, length, expiry int64) {
	d.initObject(List(nil))
}

func (d *decoder) Rpush(key, value []byte) {
	if d.err != nil {
		return
	}
	switch l := d.obj.(type) {
	default:
		d.err = fmt.Errorf("invalid object, not a list")
	case List:
		d.obj = append(l, value)
	}
}

func (d *decoder) StartZSet(key []byte, cardinality, expiry int64) {
	d.initObject(ZSet(nil))
}

func (d *decoder) Zadd(key []byte, score float64, member []byte) {
	if d.err != nil {
		return
	}
	switch z := d.obj.(type) {
	default:
		d.err = fmt.Errorf("invalid object, not a zset")
	case ZSet:
		v := struct {
			Member []byte
			Score  float64
		}{
			member,
			score,
		}
		d.obj = append(z, v)
	}
}

type String []byte
type List [][]byte
type Hash []struct {
	Field, Value []byte
}
type Set [][]byte
type ZSet []struct {
	Member []byte
	Score  float64
}
