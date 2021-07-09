package rdb

import (
	"bytes"
	"fmt"
	"github.com/cupcake/rdb"
)

func Dump(obj interface{}) ([]byte, error) {
	var buf bytes.Buffer

	e := rdb.NewEncoder(&buf)

	switch v := obj.(type) {
	case String:
		e.EncodeType(rdb.TypeString)
		e.EncodeString(v)
	case Hash:
		e.EncodeType(rdb.TypeHash)
		e.EncodeLength(uint32(len(v)))

		for i := 0; i < len(v); i++ {
			e.EncodeString(v[i].Field)
			e.EncodeString(v[i].Value)
		}
	case List:
		e.EncodeType(rdb.TypeList)
		e.EncodeLength(uint32(len(v)))
		for i := 0; i < len(v); i++ {
			e.EncodeString(v[i])
		}
	case Set:
		e.EncodeType(rdb.TypeSet)
		e.EncodeLength(uint32(len(v)))
		for i := 0; i < len(v); i++ {
			e.EncodeString(v[i])
		}
	case ZSet:
		e.EncodeType(rdb.TypeZSet)
		e.EncodeLength(uint32(len(v)))
		for i := 0; i < len(v); i++ {
			e.EncodeString(v[i].Member)
			e.EncodeFloat(v[i].Score)
		}
	default:
		return nil, fmt.Errorf("invalid dump type %T", obj)
	}

	e.EncodeDumpFooter()

	return buf.Bytes(), nil
}
