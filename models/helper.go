// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/binary"

	jsoniter "github.com/json-iterator/go"
)

func keysInt64(m map[int64]struct{}) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func valuesRepository(m map[int64]*Repository) []*Repository {
	values := make([]*Repository, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func valuesUser(m map[int64]*User) []*User {
	values := make([]*User, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// JSONUnmarshalHandleDoubleEncode - due to a bug in xorm (see https://gitea.com/xorm/xorm/pulls/1957) - it's
// possible that a Blob may be double encoded or gain an unwanted prefix of 0xff 0xfe.
func JSONUnmarshalHandleDoubleEncode(bs []byte, v interface{}) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(bs, v)
	if err != nil {
		ok := true
		rs := []byte{}
		temp := make([]byte, 2)
		for _, rn := range string(bs) {
			if rn > 0xffff {
				ok = false
				break
			}
			binary.LittleEndian.PutUint16(temp, uint16(rn))
			rs = append(rs, temp...)
		}
		if ok {
			if rs[0] == 0xff && rs[1] == 0xfe {
				rs = rs[2:]
			}
			err = json.Unmarshal(rs, v)
		}
	}
	if err != nil && len(bs) > 2 && bs[0] == 0xff && bs[1] == 0xfe {
		err = json.Unmarshal(bs[2:], v)
	}
	return err
}
