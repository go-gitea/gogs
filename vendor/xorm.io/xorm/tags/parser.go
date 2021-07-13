// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tags

import (
	"encoding/gob"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"xorm.io/xorm/caches"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

var (
	// ErrUnsupportedType represents an unsupported type error
	ErrUnsupportedType = errors.New("unsupported type")
)

// Parser represents a parser for xorm tag
type Parser struct {
	identifier   string
	dialect      dialects.Dialect
	columnMapper names.Mapper
	tableMapper  names.Mapper
	handlers     map[string]Handler
	cacherMgr    *caches.Manager
	tableCache   sync.Map // map[reflect.Type]*schemas.Table
}

// NewParser creates a tag parser
func NewParser(identifier string, dialect dialects.Dialect, tableMapper, columnMapper names.Mapper, cacherMgr *caches.Manager) *Parser {
	return &Parser{
		identifier:   identifier,
		dialect:      dialect,
		tableMapper:  tableMapper,
		columnMapper: columnMapper,
		handlers:     defaultTagHandlers,
		cacherMgr:    cacherMgr,
	}
}

// GetTableMapper returns table mapper
func (parser *Parser) GetTableMapper() names.Mapper {
	return parser.tableMapper
}

// SetTableMapper sets table mapper
func (parser *Parser) SetTableMapper(mapper names.Mapper) {
	parser.ClearCaches()
	parser.tableMapper = mapper
}

// GetColumnMapper returns column mapper
func (parser *Parser) GetColumnMapper() names.Mapper {
	return parser.columnMapper
}

// SetColumnMapper sets column mapper
func (parser *Parser) SetColumnMapper(mapper names.Mapper) {
	parser.ClearCaches()
	parser.columnMapper = mapper
}

// SetIdentifier sets tag identifier
func (parser *Parser) SetIdentifier(identifier string) {
	parser.ClearCaches()
	parser.identifier = identifier
}

// ParseWithCache parse a struct with cache
func (parser *Parser) ParseWithCache(v reflect.Value) (*schemas.Table, error) {
	t := v.Type()
	tableI, ok := parser.tableCache.Load(t)
	if ok {
		return tableI.(*schemas.Table), nil
	}

	table, err := parser.Parse(v)
	if err != nil {
		return nil, err
	}

	parser.tableCache.Store(t, table)

	if parser.cacherMgr.GetDefaultCacher() != nil {
		if v.CanAddr() {
			gob.Register(v.Addr().Interface())
		} else {
			gob.Register(v.Interface())
		}
	}

	return table, nil
}

// ClearCacheTable removes the database mapper of a type from the cache
func (parser *Parser) ClearCacheTable(t reflect.Type) {
	parser.tableCache.Delete(t)
}

// ClearCaches removes all the cached table information parsed by structs
func (parser *Parser) ClearCaches() {
	parser.tableCache = sync.Map{}
}

func addIndex(indexName string, table *schemas.Table, col *schemas.Column, indexType int) {
	if index, ok := table.Indexes[indexName]; ok {
		index.AddColumn(col.Name)
		col.Indexes[index.Name] = indexType
	} else {
		index := schemas.NewIndex(indexName, indexType)
		index.AddColumn(col.Name)
		table.AddIndex(index)
		col.Indexes[index.Name] = indexType
	}
}

var ErrIgnoreField = errors.New("field will be ignored")

func (parser *Parser) parseFieldWithNoTag(fieldIndex int, field reflect.StructField, fieldValue reflect.Value) (*schemas.Column, error) {
	var sqlType schemas.SQLType
	if fieldValue.CanAddr() {
		if _, ok := fieldValue.Addr().Interface().(convert.Conversion); ok {
			sqlType = schemas.SQLType{Name: schemas.Text}
		}
	}
	if _, ok := fieldValue.Interface().(convert.Conversion); ok {
		sqlType = schemas.SQLType{Name: schemas.Text}
	} else {
		sqlType = schemas.Type2SQLType(field.Type)
	}
	col := schemas.NewColumn(parser.columnMapper.Obj2Table(field.Name),
		field.Name, sqlType, sqlType.DefaultLength,
		sqlType.DefaultLength2, true)
	col.FieldIndex = []int{fieldIndex}

	if field.Type.Kind() == reflect.Int64 && (strings.ToUpper(col.FieldName) == "ID" || strings.HasSuffix(strings.ToUpper(col.FieldName), ".ID")) {
		col.IsAutoIncrement = true
		col.IsPrimaryKey = true
		col.Nullable = false
	}
	return col, nil
}

func (parser *Parser) parseFieldWithTags(table *schemas.Table, fieldIndex int, field reflect.StructField, fieldValue reflect.Value, tags []tag) (*schemas.Column, error) {
	var col = &schemas.Column{
		FieldName:       field.Name,
		FieldIndex:      []int{fieldIndex},
		Nullable:        true,
		IsPrimaryKey:    false,
		IsAutoIncrement: false,
		MapType:         schemas.TWOSIDES,
		Indexes:         make(map[string]int),
		DefaultIsEmpty:  true,
	}

	var ctx = Context{
		table:      table,
		col:        col,
		fieldValue: fieldValue,
		indexNames: make(map[string]int),
		parser:     parser,
	}

	for j, tag := range tags {
		if ctx.ignoreNext {
			ctx.ignoreNext = false
			continue
		}

		ctx.tag = tag
		ctx.tagUname = strings.ToUpper(tag.name)

		if j > 0 {
			ctx.preTag = strings.ToUpper(tags[j-1].name)
		}
		if j < len(tags)-1 {
			ctx.nextTag = tags[j+1].name
		} else {
			ctx.nextTag = ""
		}

		if h, ok := parser.handlers[ctx.tagUname]; ok {
			if err := h(&ctx); err != nil {
				return nil, err
			}
		} else {
			if strings.HasPrefix(ctx.tag.name, "'") && strings.HasSuffix(ctx.tag.name, "'") {
				col.Name = ctx.tag.name[1 : len(ctx.tag.name)-1]
			} else {
				col.Name = ctx.tag.name
			}
		}

		if ctx.hasCacheTag {
			if parser.cacherMgr.GetDefaultCacher() != nil {
				parser.cacherMgr.SetCacher(table.Name, parser.cacherMgr.GetDefaultCacher())
			} else {
				parser.cacherMgr.SetCacher(table.Name, caches.NewLRUCacher2(caches.NewMemoryStore(), time.Hour, 10000))
			}
		}
		if ctx.hasNoCacheTag {
			parser.cacherMgr.SetCacher(table.Name, nil)
		}
	}

	if col.SQLType.Name == "" {
		col.SQLType = schemas.Type2SQLType(field.Type)
	}
	parser.dialect.SQLType(col)
	if col.Length == 0 {
		col.Length = col.SQLType.DefaultLength
	}
	if col.Length2 == 0 {
		col.Length2 = col.SQLType.DefaultLength2
	}
	if col.Name == "" {
		col.Name = parser.columnMapper.Obj2Table(field.Name)
	}

	if ctx.isUnique {
		ctx.indexNames[col.Name] = schemas.UniqueType
	} else if ctx.isIndex {
		ctx.indexNames[col.Name] = schemas.IndexType
	}

	for indexName, indexType := range ctx.indexNames {
		addIndex(indexName, table, col, indexType)
	}

	return col, nil
}

func (parser *Parser) parseField(table *schemas.Table, fieldIndex int, field reflect.StructField, fieldValue reflect.Value) (*schemas.Column, error) {
	var (
		tag       = field.Tag
		ormTagStr = strings.TrimSpace(tag.Get(parser.identifier))
	)
	if ormTagStr == "-" {
		return nil, ErrIgnoreField
	}
	if ormTagStr == "" {
		return parser.parseFieldWithNoTag(fieldIndex, field, fieldValue)
	}
	tags, err := splitTag(ormTagStr)
	if err != nil {
		return nil, err
	}
	return parser.parseFieldWithTags(table, fieldIndex, field, fieldValue, tags)
}

func isNotTitle(n string) bool {
	for _, c := range n {
		return unicode.IsLower(c)
	}
	return true
}

// Parse parses a struct as a table information
func (parser *Parser) Parse(v reflect.Value) (*schemas.Table, error) {
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, ErrUnsupportedType
	}

	table := schemas.NewEmptyTable()
	table.Type = t
	table.Name = names.GetTableName(parser.tableMapper, v)

	for i := 0; i < t.NumField(); i++ {
		var field = t.Field(i)
		if isNotTitle(field.Name) {
			continue
		}

		col, err := parser.parseField(table, i, field, v.Field(i))
		if err == ErrIgnoreField {
			continue
		} else if err != nil {
			return nil, err
		}

		table.AddColumn(col)
	} // end for

	deletedColumn := table.DeletedColumn()
	// check columns
	if deletedColumn != nil {
		deletedColumn.Nullable = true
	}

	return table, nil
}
