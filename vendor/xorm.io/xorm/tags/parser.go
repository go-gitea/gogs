// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tags

import (
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"xorm.io/xorm/caches"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

var (
	ErrUnsupportedType = errors.New("Unsupported type")
)

type Parser struct {
	identifier   string
	dialect      dialects.Dialect
	columnMapper names.Mapper
	tableMapper  names.Mapper
	handlers     map[string]Handler
	cacherMgr    *caches.Manager
	tableCache   sync.Map // map[reflect.Type]*schemas.Table
}

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

func (parser *Parser) GetTableMapper() names.Mapper {
	return parser.tableMapper
}

func (parser *Parser) SetTableMapper(mapper names.Mapper) {
	parser.ClearCaches()
	parser.tableMapper = mapper
}

func (parser *Parser) GetColumnMapper() names.Mapper {
	return parser.columnMapper
}

func (parser *Parser) SetColumnMapper(mapper names.Mapper) {
	parser.ClearCaches()
	parser.columnMapper = mapper
}

func (parser *Parser) SetIdentifier(identifier string) {
	parser.ClearCaches()
	parser.identifier = identifier
}

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

	var idFieldColName string
	var hasCacheTag, hasNoCacheTag bool

	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag

		ormTagStr := tag.Get(parser.identifier)
		var col *schemas.Column
		fieldValue := v.Field(i)
		fieldType := fieldValue.Type()

		if ormTagStr != "" {
			col = &schemas.Column{
				FieldName:       t.Field(i).Name,
				Nullable:        true,
				IsPrimaryKey:    false,
				IsAutoIncrement: false,
				MapType:         schemas.TWOSIDES,
				Indexes:         make(map[string]int),
				DefaultIsEmpty:  true,
			}
			tags := splitTag(ormTagStr)

			if len(tags) > 0 {
				if tags[0] == "-" {
					continue
				}

				var ctx = Context{
					table:      table,
					col:        col,
					fieldValue: fieldValue,
					indexNames: make(map[string]int),
					parser:     parser,
				}

				if strings.HasPrefix(strings.ToUpper(tags[0]), "EXTENDS") {
					pStart := strings.Index(tags[0], "(")
					if pStart > -1 && strings.HasSuffix(tags[0], ")") {
						var tagPrefix = strings.TrimFunc(tags[0][pStart+1:len(tags[0])-1], func(r rune) bool {
							return r == '\'' || r == '"'
						})

						ctx.params = []string{tagPrefix}
					}

					if err := ExtendsTagHandler(&ctx); err != nil {
						return nil, err
					}
					continue
				}

				for j, key := range tags {
					if ctx.ignoreNext {
						ctx.ignoreNext = false
						continue
					}

					k := strings.ToUpper(key)
					ctx.tagName = k
					ctx.params = []string{}

					pStart := strings.Index(k, "(")
					if pStart == 0 {
						return nil, errors.New("( could not be the first character")
					}
					if pStart > -1 {
						if !strings.HasSuffix(k, ")") {
							return nil, fmt.Errorf("field %s tag %s cannot match ) character", col.FieldName, key)
						}

						ctx.tagName = k[:pStart]
						ctx.params = strings.Split(key[pStart+1:len(k)-1], ",")
					}

					if j > 0 {
						ctx.preTag = strings.ToUpper(tags[j-1])
					}
					if j < len(tags)-1 {
						ctx.nextTag = tags[j+1]
					} else {
						ctx.nextTag = ""
					}

					if h, ok := parser.handlers[ctx.tagName]; ok {
						if err := h(&ctx); err != nil {
							return nil, err
						}
					} else {
						if strings.HasPrefix(key, "'") && strings.HasSuffix(key, "'") {
							col.Name = key[1 : len(key)-1]
						} else {
							col.Name = key
						}
					}

					if ctx.hasCacheTag {
						hasCacheTag = true
					}
					if ctx.hasNoCacheTag {
						hasNoCacheTag = true
					}
				}

				if col.SQLType.Name == "" {
					col.SQLType = schemas.Type2SQLType(fieldType)
				}
				parser.dialect.SQLType(col)
				if col.Length == 0 {
					col.Length = col.SQLType.DefaultLength
				}
				if col.Length2 == 0 {
					col.Length2 = col.SQLType.DefaultLength2
				}
				if col.Name == "" {
					col.Name = parser.columnMapper.Obj2Table(t.Field(i).Name)
				}

				if ctx.isUnique {
					ctx.indexNames[col.Name] = schemas.UniqueType
				} else if ctx.isIndex {
					ctx.indexNames[col.Name] = schemas.IndexType
				}

				for indexName, indexType := range ctx.indexNames {
					addIndex(indexName, table, col, indexType)
				}
			}
		} else if fieldValue.CanSet() {
			var sqlType schemas.SQLType
			if fieldValue.CanAddr() {
				if _, ok := fieldValue.Addr().Interface().(convert.Conversion); ok {
					sqlType = schemas.SQLType{Name: schemas.Text}
				}
			}
			if _, ok := fieldValue.Interface().(convert.Conversion); ok {
				sqlType = schemas.SQLType{Name: schemas.Text}
			} else {
				sqlType = schemas.Type2SQLType(fieldType)
			}
			col = schemas.NewColumn(parser.columnMapper.Obj2Table(t.Field(i).Name),
				t.Field(i).Name, sqlType, sqlType.DefaultLength,
				sqlType.DefaultLength2, true)

			if fieldType.Kind() == reflect.Int64 && (strings.ToUpper(col.FieldName) == "ID" || strings.HasSuffix(strings.ToUpper(col.FieldName), ".ID")) {
				idFieldColName = col.Name
			}
		} else {
			continue
		}
		if col.IsAutoIncrement {
			col.Nullable = false
		}

		table.AddColumn(col)

	} // end for

	if idFieldColName != "" && len(table.PrimaryKeys) == 0 {
		col := table.GetColumn(idFieldColName)
		col.IsPrimaryKey = true
		col.IsAutoIncrement = true
		col.Nullable = false
		table.PrimaryKeys = append(table.PrimaryKeys, col.Name)
		table.AutoIncrement = col.Name
	}

	if hasCacheTag {
		if parser.cacherMgr.GetDefaultCacher() != nil { // !nash! use engine's cacher if provided
			//engine.logger.Info("enable cache on table:", table.Name)
			parser.cacherMgr.SetCacher(table.Name, parser.cacherMgr.GetDefaultCacher())
		} else {
			//engine.logger.Info("enable LRU cache on table:", table.Name)
			parser.cacherMgr.SetCacher(table.Name, caches.NewLRUCacher2(caches.NewMemoryStore(), time.Hour, 10000))
		}
	}
	if hasNoCacheTag {
		//engine.logger.Info("disable cache on table:", table.Name)
		parser.cacherMgr.SetCacher(table.Name, nil)
	}

	return table, nil
}
