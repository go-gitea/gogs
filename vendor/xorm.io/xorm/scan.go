// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"database/sql"

	"xorm.io/xorm/core"
)

func (engine *Engine) row2mapStr(rows *core.Rows, types []*sql.ColumnType, fields []string) (map[string]string, error) {
	var scanResults = make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var s sql.NullString
		scanResults[i] = &s
	}

	if err := rows.Scan(scanResults...); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(fields))
	for ii, key := range fields {
		s := scanResults[ii].(*sql.NullString)
		result[key] = s.String
	}
	return result, nil
}

func (engine *Engine) row2mapBytes(rows *core.Rows, types []*sql.ColumnType, fields []string) (map[string][]byte, error) {
	var scanResults = make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var s sql.NullString
		scanResults[i] = &s
	}

	if err := rows.Scan(scanResults...); err != nil {
		return nil, err
	}

	result := make(map[string][]byte, len(fields))
	for ii, key := range fields {
		s := scanResults[ii].(*sql.NullString)
		result[key] = []byte(s.String)
	}
	return result, nil
}

func (engine *Engine) row2sliceStr(rows *core.Rows, types []*sql.ColumnType, fields []string) ([]string, error) {
	results := make([]string, 0, len(fields))
	var scanResults = make([]interface{}, len(fields))
	for i := 0; i < len(fields); i++ {
		var s sql.NullString
		scanResults[i] = &s
	}

	if err := rows.Scan(scanResults...); err != nil {
		return nil, err
	}

	for i := 0; i < len(fields); i++ {
		results = append(results, scanResults[i].(*sql.NullString).String)
	}
	return results, nil
}
