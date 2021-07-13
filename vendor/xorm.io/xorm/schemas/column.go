// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schemas

import (
	"errors"
	"reflect"
	"strconv"
	"time"
)

// enumerates all database mapping way
const (
	TWOSIDES = iota + 1
	ONLYTODB
	ONLYFROMDB
)

// Column defines database column
type Column struct {
	Name            string
	TableName       string
	FieldName       string // Available only when parsed from a struct
	FieldIndex      []int  // Available only when parsed from a struct
	SQLType         SQLType
	IsJSON          bool
	Length          int
	Length2         int
	Nullable        bool
	Default         string
	Indexes         map[string]int
	IsPrimaryKey    bool
	IsAutoIncrement bool
	MapType         int
	IsCreated       bool
	IsUpdated       bool
	IsDeleted       bool
	IsCascade       bool
	IsVersion       bool
	DefaultIsEmpty  bool // false means column has no default set, but not default value is empty
	EnumOptions     map[string]int
	SetOptions      map[string]int
	DisableTimeZone bool
	TimeZone        *time.Location // column specified time zone
	Comment         string
}

// NewColumn creates a new column
func NewColumn(name, fieldName string, sqlType SQLType, len1, len2 int, nullable bool) *Column {
	return &Column{
		Name:            name,
		IsJSON:          sqlType.IsJson(),
		TableName:       "",
		FieldName:       fieldName,
		SQLType:         sqlType,
		Length:          len1,
		Length2:         len2,
		Nullable:        nullable,
		Default:         "",
		Indexes:         make(map[string]int),
		IsPrimaryKey:    false,
		IsAutoIncrement: false,
		MapType:         TWOSIDES,
		IsCreated:       false,
		IsUpdated:       false,
		IsDeleted:       false,
		IsCascade:       false,
		IsVersion:       false,
		DefaultIsEmpty:  true, // default should be no default
		EnumOptions:     make(map[string]int),
		Comment:         "",
	}
}

// ValueOf returns column's filed of struct's value
func (col *Column) ValueOf(bean interface{}) (*reflect.Value, error) {
	dataStruct := reflect.Indirect(reflect.ValueOf(bean))
	return col.ValueOfV(&dataStruct)
}

// ValueOfV returns column's filed of struct's value accept reflevt value
func (col *Column) ValueOfV(dataStruct *reflect.Value) (*reflect.Value, error) {
	var v = *dataStruct
	for _, i := range col.FieldIndex {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.FieldByIndex([]int{i})
	}
	return &v, nil
}

// ConvertID converts id content to suitable type according column type
func (col *Column) ConvertID(sid string) (interface{}, error) {
	if col.SQLType.IsNumeric() {
		n, err := strconv.ParseInt(sid, 10, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	} else if col.SQLType.IsText() {
		return sid, nil
	}
	return nil, errors.New("not supported")
}
