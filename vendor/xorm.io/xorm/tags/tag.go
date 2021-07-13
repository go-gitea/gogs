// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tags

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"xorm.io/xorm/schemas"
)

type tag struct {
	name   string
	params []string
}

func splitTag(tagStr string) ([]tag, error) {
	tagStr = strings.TrimSpace(tagStr)
	var (
		inQuote    bool
		inBigQuote bool
		lastIdx    int
		curTag     tag
		paramStart int
		tags       []tag
	)
	for i, t := range tagStr {
		switch t {
		case '\'':
			inQuote = !inQuote
		case ' ':
			if !inQuote && !inBigQuote {
				if lastIdx < i {
					if curTag.name == "" {
						curTag.name = tagStr[lastIdx:i]
					}
					tags = append(tags, curTag)
					lastIdx = i + 1
					curTag = tag{}
				} else if lastIdx == i {
					lastIdx = i + 1
				}
			} else if inBigQuote && !inQuote {
				paramStart = i + 1
			}
		case ',':
			if !inQuote && !inBigQuote {
				return nil, fmt.Errorf("comma[%d] of %s should be in quote or big quote", i, tagStr)
			}
			if !inQuote && inBigQuote {
				curTag.params = append(curTag.params, strings.TrimSpace(tagStr[paramStart:i]))
				paramStart = i + 1
			}
		case '(':
			inBigQuote = true
			if !inQuote {
				curTag.name = tagStr[lastIdx:i]
				paramStart = i + 1
			}
		case ')':
			inBigQuote = false
			if !inQuote {
				curTag.params = append(curTag.params, tagStr[paramStart:i])
			}
		}
	}
	if lastIdx < len(tagStr) {
		if curTag.name == "" {
			curTag.name = tagStr[lastIdx:]
		}
		tags = append(tags, curTag)
	}
	return tags, nil
}

// Context represents a context for xorm tag parse.
type Context struct {
	tag
	tagUname        string
	preTag, nextTag string
	table           *schemas.Table
	col             *schemas.Column
	fieldValue      reflect.Value
	isIndex         bool
	isUnique        bool
	indexNames      map[string]int
	parser          *Parser
	hasCacheTag     bool
	hasNoCacheTag   bool
	ignoreNext      bool
}

// Handler describes tag handler for XORM
type Handler func(ctx *Context) error

var (
	// defaultTagHandlers enumerates all the default tag handler
	defaultTagHandlers = map[string]Handler{
		"<-":       OnlyFromDBTagHandler,
		"->":       OnlyToDBTagHandler,
		"PK":       PKTagHandler,
		"NULL":     NULLTagHandler,
		"NOT":      IgnoreTagHandler,
		"AUTOINCR": AutoIncrTagHandler,
		"DEFAULT":  DefaultTagHandler,
		"CREATED":  CreatedTagHandler,
		"UPDATED":  UpdatedTagHandler,
		"DELETED":  DeletedTagHandler,
		"VERSION":  VersionTagHandler,
		"UTC":      UTCTagHandler,
		"LOCAL":    LocalTagHandler,
		"NOTNULL":  NotNullTagHandler,
		"INDEX":    IndexTagHandler,
		"UNIQUE":   UniqueTagHandler,
		"CACHE":    CacheTagHandler,
		"NOCACHE":  NoCacheTagHandler,
		"COMMENT":  CommentTagHandler,
		"EXTENDS":  ExtendsTagHandler,
	}
)

func init() {
	for k := range schemas.SqlTypes {
		defaultTagHandlers[k] = SQLTypeTagHandler
	}
}

// IgnoreTagHandler describes ignored tag handler
func IgnoreTagHandler(ctx *Context) error {
	return nil
}

// OnlyFromDBTagHandler describes mapping direction tag handler
func OnlyFromDBTagHandler(ctx *Context) error {
	ctx.col.MapType = schemas.ONLYFROMDB
	return nil
}

// OnlyToDBTagHandler describes mapping direction tag handler
func OnlyToDBTagHandler(ctx *Context) error {
	ctx.col.MapType = schemas.ONLYTODB
	return nil
}

// PKTagHandler describes primary key tag handler
func PKTagHandler(ctx *Context) error {
	ctx.col.IsPrimaryKey = true
	ctx.col.Nullable = false
	return nil
}

// NULLTagHandler describes null tag handler
func NULLTagHandler(ctx *Context) error {
	ctx.col.Nullable = (strings.ToUpper(ctx.preTag) != "NOT")
	return nil
}

// NotNullTagHandler describes notnull tag handler
func NotNullTagHandler(ctx *Context) error {
	ctx.col.Nullable = false
	return nil
}

// AutoIncrTagHandler describes autoincr tag handler
func AutoIncrTagHandler(ctx *Context) error {
	ctx.col.IsAutoIncrement = true
	ctx.col.Nullable = false
	/*
		if len(ctx.params) > 0 {
			autoStartInt, err := strconv.Atoi(ctx.params[0])
			if err != nil {
				return err
			}
			ctx.col.AutoIncrStart = autoStartInt
		} else {
			ctx.col.AutoIncrStart = 1
		}
	*/
	return nil
}

// DefaultTagHandler describes default tag handler
func DefaultTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.col.Default = ctx.params[0]
	} else {
		ctx.col.Default = ctx.nextTag
		ctx.ignoreNext = true
	}
	ctx.col.DefaultIsEmpty = false
	return nil
}

// CreatedTagHandler describes created tag handler
func CreatedTagHandler(ctx *Context) error {
	ctx.col.IsCreated = true
	return nil
}

// VersionTagHandler describes version tag handler
func VersionTagHandler(ctx *Context) error {
	ctx.col.IsVersion = true
	ctx.col.Default = "1"
	return nil
}

// UTCTagHandler describes utc tag handler
func UTCTagHandler(ctx *Context) error {
	ctx.col.TimeZone = time.UTC
	return nil
}

// LocalTagHandler describes local tag handler
func LocalTagHandler(ctx *Context) error {
	if len(ctx.params) == 0 {
		ctx.col.TimeZone = time.Local
	} else {
		var err error
		ctx.col.TimeZone, err = time.LoadLocation(ctx.params[0])
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdatedTagHandler describes updated tag handler
func UpdatedTagHandler(ctx *Context) error {
	ctx.col.IsUpdated = true
	return nil
}

// DeletedTagHandler describes deleted tag handler
func DeletedTagHandler(ctx *Context) error {
	ctx.col.IsDeleted = true
	ctx.col.Nullable = true
	return nil
}

// IndexTagHandler describes index tag handler
func IndexTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.indexNames[ctx.params[0]] = schemas.IndexType
	} else {
		ctx.isIndex = true
	}
	return nil
}

// UniqueTagHandler describes unique tag handler
func UniqueTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.indexNames[ctx.params[0]] = schemas.UniqueType
	} else {
		ctx.isUnique = true
	}
	return nil
}

// CommentTagHandler add comment to column
func CommentTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.col.Comment = strings.Trim(ctx.params[0], "' ")
	}
	return nil
}

// SQLTypeTagHandler describes SQL Type tag handler
func SQLTypeTagHandler(ctx *Context) error {
	ctx.col.SQLType = schemas.SQLType{Name: ctx.tagUname}
	if ctx.tagUname == "JSON" {
		ctx.col.IsJSON = true
	}
	if len(ctx.params) == 0 {
		return nil
	}

	switch ctx.tagUname {
	case schemas.Enum:
		ctx.col.EnumOptions = make(map[string]int)
		for k, v := range ctx.params {
			v = strings.TrimSpace(v)
			v = strings.Trim(v, "'")
			ctx.col.EnumOptions[v] = k
		}
	case schemas.Set:
		ctx.col.SetOptions = make(map[string]int)
		for k, v := range ctx.params {
			v = strings.TrimSpace(v)
			v = strings.Trim(v, "'")
			ctx.col.SetOptions[v] = k
		}
	default:
		var err error
		if len(ctx.params) == 2 {
			ctx.col.Length, err = strconv.Atoi(ctx.params[0])
			if err != nil {
				return err
			}
			ctx.col.Length2, err = strconv.Atoi(ctx.params[1])
			if err != nil {
				return err
			}
		} else if len(ctx.params) == 1 {
			ctx.col.Length, err = strconv.Atoi(ctx.params[0])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ExtendsTagHandler describes extends tag handler
func ExtendsTagHandler(ctx *Context) error {
	var fieldValue = ctx.fieldValue
	var isPtr = false
	switch fieldValue.Kind() {
	case reflect.Ptr:
		f := fieldValue.Type().Elem()
		if f.Kind() == reflect.Struct {
			fieldPtr := fieldValue
			fieldValue = fieldValue.Elem()
			if !fieldValue.IsValid() || fieldPtr.IsNil() {
				fieldValue = reflect.New(f).Elem()
			}
		}
		isPtr = true
		fallthrough
	case reflect.Struct:
		parentTable, err := ctx.parser.Parse(fieldValue)
		if err != nil {
			return err
		}
		for _, col := range parentTable.Columns() {
			col.FieldName = fmt.Sprintf("%v.%v", ctx.col.FieldName, col.FieldName)
			col.FieldIndex = append(ctx.col.FieldIndex, col.FieldIndex...)

			var tagPrefix = ctx.col.FieldName
			if len(ctx.params) > 0 {
				col.Nullable = isPtr
				tagPrefix = strings.Trim(ctx.params[0], "'")
				if col.IsPrimaryKey {
					col.Name = ctx.col.FieldName
					col.IsPrimaryKey = false
				} else {
					col.Name = fmt.Sprintf("%v%v", tagPrefix, col.Name)
				}
			}

			if col.Nullable {
				col.IsAutoIncrement = false
				col.IsPrimaryKey = false
			}

			ctx.table.AddColumn(col)
			for indexName, indexType := range col.Indexes {
				addIndex(indexName, ctx.table, col, indexType)
			}
		}
	default:
		//TODO: warning
	}
	return ErrIgnoreField
}

// CacheTagHandler describes cache tag handler
func CacheTagHandler(ctx *Context) error {
	if !ctx.hasCacheTag {
		ctx.hasCacheTag = true
	}
	return nil
}

// NoCacheTagHandler describes nocache tag handler
func NoCacheTagHandler(ctx *Context) error {
	if !ctx.hasNoCacheTag {
		ctx.hasNoCacheTag = true
	}
	return nil
}
