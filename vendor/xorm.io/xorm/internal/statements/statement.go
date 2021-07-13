// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"xorm.io/builder"
	"xorm.io/xorm/contexts"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/internal/json"
	"xorm.io/xorm/internal/utils"
	"xorm.io/xorm/schemas"
	"xorm.io/xorm/tags"
)

var (
	// ErrConditionType condition type unsupported
	ErrConditionType = errors.New("Unsupported condition type")
	// ErrUnSupportedSQLType parameter of SQL is not supported
	ErrUnSupportedSQLType = errors.New("Unsupported sql type")
	// ErrUnSupportedType unsupported error
	ErrUnSupportedType = errors.New("Unsupported type error")
	// ErrTableNotFound table not found error
	ErrTableNotFound = errors.New("Table not found")
)

// Statement save all the sql info for executing SQL
type Statement struct {
	RefTable        *schemas.Table
	dialect         dialects.Dialect
	defaultTimeZone *time.Location
	tagParser       *tags.Parser
	Start           int
	LimitN          *int
	idParam         schemas.PK
	OrderStr        string
	JoinStr         string
	joinArgs        []interface{}
	GroupByStr      string
	HavingStr       string
	SelectStr       string
	useAllCols      bool
	AltTableName    string
	tableName       string
	RawSQL          string
	RawParams       []interface{}
	UseCascade      bool
	UseAutoJoin     bool
	StoreEngine     string
	Charset         string
	UseCache        bool
	UseAutoTime     bool
	NoAutoCondition bool
	IsDistinct      bool
	IsForUpdate     bool
	TableAlias      string
	allUseBool      bool
	CheckVersion    bool
	unscoped        bool
	ColumnMap       columnMap
	OmitColumnMap   columnMap
	MustColumnMap   map[string]bool
	NullableMap     map[string]bool
	IncrColumns     exprParams
	DecrColumns     exprParams
	ExprColumns     exprParams
	cond            builder.Cond
	BufferSize      int
	Context         contexts.ContextCache
	LastError       error
}

// NewStatement creates a new statement
func NewStatement(dialect dialects.Dialect, tagParser *tags.Parser, defaultTimeZone *time.Location) *Statement {
	statement := &Statement{
		dialect:         dialect,
		tagParser:       tagParser,
		defaultTimeZone: defaultTimeZone,
	}
	statement.Reset()
	return statement
}

// SetTableName set table name
func (statement *Statement) SetTableName(tableName string) {
	statement.tableName = tableName
}

// GenRawSQL generates correct raw sql
func (statement *Statement) GenRawSQL() string {
	return statement.ReplaceQuote(statement.RawSQL)
}

// GenCondSQL generates condition SQL
func (statement *Statement) GenCondSQL(condOrBuilder interface{}) (string, []interface{}, error) {
	condSQL, condArgs, err := builder.ToSQL(condOrBuilder)
	if err != nil {
		return "", nil, err
	}
	return statement.ReplaceQuote(condSQL), condArgs, nil
}

// ReplaceQuote replace sql key words with quote
func (statement *Statement) ReplaceQuote(sql string) string {
	if sql == "" || statement.dialect.URI().DBType == schemas.MYSQL ||
		statement.dialect.URI().DBType == schemas.SQLITE {
		return sql
	}
	return statement.dialect.Quoter().Replace(sql)
}

// SetContextCache sets context cache
func (statement *Statement) SetContextCache(ctxCache contexts.ContextCache) {
	statement.Context = ctxCache
}

// Reset reset all the statement's fields
func (statement *Statement) Reset() {
	statement.RefTable = nil
	statement.Start = 0
	statement.LimitN = nil
	statement.OrderStr = ""
	statement.UseCascade = true
	statement.JoinStr = ""
	statement.joinArgs = make([]interface{}, 0)
	statement.GroupByStr = ""
	statement.HavingStr = ""
	statement.ColumnMap = columnMap{}
	statement.OmitColumnMap = columnMap{}
	statement.AltTableName = ""
	statement.tableName = ""
	statement.idParam = nil
	statement.RawSQL = ""
	statement.RawParams = make([]interface{}, 0)
	statement.UseCache = true
	statement.UseAutoTime = true
	statement.NoAutoCondition = false
	statement.IsDistinct = false
	statement.IsForUpdate = false
	statement.TableAlias = ""
	statement.SelectStr = ""
	statement.allUseBool = false
	statement.useAllCols = false
	statement.MustColumnMap = make(map[string]bool)
	statement.NullableMap = make(map[string]bool)
	statement.CheckVersion = true
	statement.unscoped = false
	statement.IncrColumns = exprParams{}
	statement.DecrColumns = exprParams{}
	statement.ExprColumns = exprParams{}
	statement.cond = builder.NewCond()
	statement.BufferSize = 0
	statement.Context = nil
	statement.LastError = nil
}

// SetNoAutoCondition if you do not want convert bean's field as query condition, then use this function
func (statement *Statement) SetNoAutoCondition(no ...bool) *Statement {
	statement.NoAutoCondition = true
	if len(no) > 0 {
		statement.NoAutoCondition = no[0]
	}
	return statement
}

// Alias set the table alias
func (statement *Statement) Alias(alias string) *Statement {
	statement.TableAlias = alias
	return statement
}

// SQL adds raw sql statement
func (statement *Statement) SQL(query interface{}, args ...interface{}) *Statement {
	switch query.(type) {
	case (*builder.Builder):
		var err error
		statement.RawSQL, statement.RawParams, err = query.(*builder.Builder).ToSQL()
		if err != nil {
			statement.LastError = err
		}
	case string:
		statement.RawSQL = query.(string)
		statement.RawParams = args
	default:
		statement.LastError = ErrUnSupportedSQLType
	}

	return statement
}

// Where add Where statement
func (statement *Statement) Where(query interface{}, args ...interface{}) *Statement {
	return statement.And(query, args...)
}

func (statement *Statement) quote(s string) string {
	return statement.dialect.Quoter().Quote(s)
}

// And add Where & and statement
func (statement *Statement) And(query interface{}, args ...interface{}) *Statement {
	switch qr := query.(type) {
	case string:
		cond := builder.Expr(qr, args...)
		statement.cond = statement.cond.And(cond)
	case map[string]interface{}:
		cond := make(builder.Eq)
		for k, v := range qr {
			cond[statement.quote(k)] = v
		}
		statement.cond = statement.cond.And(cond)
	case builder.Cond:
		statement.cond = statement.cond.And(qr)
		for _, v := range args {
			if vv, ok := v.(builder.Cond); ok {
				statement.cond = statement.cond.And(vv)
			}
		}
	default:
		statement.LastError = ErrConditionType
	}

	return statement
}

// Or add Where & Or statement
func (statement *Statement) Or(query interface{}, args ...interface{}) *Statement {
	switch qr := query.(type) {
	case string:
		cond := builder.Expr(qr, args...)
		statement.cond = statement.cond.Or(cond)
	case map[string]interface{}:
		cond := make(builder.Eq)
		for k, v := range qr {
			cond[statement.quote(k)] = v
		}
		statement.cond = statement.cond.Or(cond)
	case builder.Cond:
		statement.cond = statement.cond.Or(qr)
		for _, v := range args {
			if vv, ok := v.(builder.Cond); ok {
				statement.cond = statement.cond.Or(vv)
			}
		}
	default:
		statement.LastError = ErrConditionType
	}
	return statement
}

// In generate "Where column IN (?) " statement
func (statement *Statement) In(column string, args ...interface{}) *Statement {
	in := builder.In(statement.quote(column), args...)
	statement.cond = statement.cond.And(in)
	return statement
}

// NotIn generate "Where column NOT IN (?) " statement
func (statement *Statement) NotIn(column string, args ...interface{}) *Statement {
	notIn := builder.NotIn(statement.quote(column), args...)
	statement.cond = statement.cond.And(notIn)
	return statement
}

// SetRefValue set ref value
func (statement *Statement) SetRefValue(v reflect.Value) error {
	var err error
	statement.RefTable, err = statement.tagParser.ParseWithCache(reflect.Indirect(v))
	if err != nil {
		return err
	}
	statement.tableName = dialects.FullTableName(statement.dialect, statement.tagParser.GetTableMapper(), v, true)
	return nil
}

func rValue(bean interface{}) reflect.Value {
	return reflect.Indirect(reflect.ValueOf(bean))
}

// SetRefBean set ref bean
func (statement *Statement) SetRefBean(bean interface{}) error {
	var err error
	statement.RefTable, err = statement.tagParser.ParseWithCache(rValue(bean))
	if err != nil {
		return err
	}
	statement.tableName = dialects.FullTableName(statement.dialect, statement.tagParser.GetTableMapper(), bean, true)
	return nil
}

func (statement *Statement) needTableName() bool {
	return len(statement.JoinStr) > 0
}

func (statement *Statement) colName(col *schemas.Column, tableName string) string {
	if statement.needTableName() {
		var nm = tableName
		if len(statement.TableAlias) > 0 {
			nm = statement.TableAlias
		}
		return statement.quote(nm) + "." + statement.quote(col.Name)
	}
	return statement.quote(col.Name)
}

// TableName return current tableName
func (statement *Statement) TableName() string {
	if statement.AltTableName != "" {
		return statement.AltTableName
	}

	return statement.tableName
}

// Incr Generate  "Update ... Set column = column + arg" statement
func (statement *Statement) Incr(column string, arg ...interface{}) *Statement {
	if len(arg) > 0 {
		statement.IncrColumns.Add(column, arg[0])
	} else {
		statement.IncrColumns.Add(column, 1)
	}
	return statement
}

// Decr Generate  "Update ... Set column = column - arg" statement
func (statement *Statement) Decr(column string, arg ...interface{}) *Statement {
	if len(arg) > 0 {
		statement.DecrColumns.Add(column, arg[0])
	} else {
		statement.DecrColumns.Add(column, 1)
	}
	return statement
}

// SetExpr Generate  "Update ... Set column = {expression}" statement
func (statement *Statement) SetExpr(column string, expression interface{}) *Statement {
	if e, ok := expression.(string); ok {
		statement.ExprColumns.Add(column, statement.dialect.Quoter().Replace(e))
	} else {
		statement.ExprColumns.Add(column, expression)
	}
	return statement
}

// Distinct generates "DISTINCT col1, col2 " statement
func (statement *Statement) Distinct(columns ...string) *Statement {
	statement.IsDistinct = true
	statement.Cols(columns...)
	return statement
}

// ForUpdate generates "SELECT ... FOR UPDATE" statement
func (statement *Statement) ForUpdate() *Statement {
	statement.IsForUpdate = true
	return statement
}

// Select replace select
func (statement *Statement) Select(str string) *Statement {
	statement.SelectStr = statement.ReplaceQuote(str)
	return statement
}

func col2NewCols(columns ...string) []string {
	newColumns := make([]string, 0, len(columns))
	for _, col := range columns {
		col = strings.Replace(col, "`", "", -1)
		col = strings.Replace(col, `"`, "", -1)
		ccols := strings.Split(col, ",")
		for _, c := range ccols {
			newColumns = append(newColumns, strings.TrimSpace(c))
		}
	}
	return newColumns
}

// Cols generate "col1, col2" statement
func (statement *Statement) Cols(columns ...string) *Statement {
	cols := col2NewCols(columns...)
	for _, nc := range cols {
		statement.ColumnMap.Add(nc)
	}
	return statement
}

// ColumnStr returns column string
func (statement *Statement) ColumnStr() string {
	return statement.dialect.Quoter().Join(statement.ColumnMap, ", ")
}

// AllCols update use only: update all columns
func (statement *Statement) AllCols() *Statement {
	statement.useAllCols = true
	return statement
}

// MustCols update use only: must update columns
func (statement *Statement) MustCols(columns ...string) *Statement {
	newColumns := col2NewCols(columns...)
	for _, nc := range newColumns {
		statement.MustColumnMap[strings.ToLower(nc)] = true
	}
	return statement
}

// UseBool indicates that use bool fields as update contents and query contiditions
func (statement *Statement) UseBool(columns ...string) *Statement {
	if len(columns) > 0 {
		statement.MustCols(columns...)
	} else {
		statement.allUseBool = true
	}
	return statement
}

// Omit do not use the columns
func (statement *Statement) Omit(columns ...string) {
	newColumns := col2NewCols(columns...)
	for _, nc := range newColumns {
		statement.OmitColumnMap = append(statement.OmitColumnMap, nc)
	}
}

// Nullable Update use only: update columns to null when value is nullable and zero-value
func (statement *Statement) Nullable(columns ...string) {
	newColumns := col2NewCols(columns...)
	for _, nc := range newColumns {
		statement.NullableMap[strings.ToLower(nc)] = true
	}
}

// Top generate LIMIT limit statement
func (statement *Statement) Top(limit int) *Statement {
	statement.Limit(limit)
	return statement
}

// Limit generate LIMIT start, limit statement
func (statement *Statement) Limit(limit int, start ...int) *Statement {
	statement.LimitN = &limit
	if len(start) > 0 {
		statement.Start = start[0]
	}
	return statement
}

// OrderBy generate "Order By order" statement
func (statement *Statement) OrderBy(order string) *Statement {
	if len(statement.OrderStr) > 0 {
		statement.OrderStr += ", "
	}
	statement.OrderStr += statement.ReplaceQuote(order)
	return statement
}

// Desc generate `ORDER BY xx DESC`
func (statement *Statement) Desc(colNames ...string) *Statement {
	var buf strings.Builder
	if len(statement.OrderStr) > 0 {
		fmt.Fprint(&buf, statement.OrderStr, ", ")
	}
	for i, col := range colNames {
		if i > 0 {
			fmt.Fprint(&buf, ", ")
		}
		statement.dialect.Quoter().QuoteTo(&buf, col)
		fmt.Fprint(&buf, " DESC")
	}
	statement.OrderStr = buf.String()
	return statement
}

// Asc provide asc order by query condition, the input parameters are columns.
func (statement *Statement) Asc(colNames ...string) *Statement {
	var buf strings.Builder
	if len(statement.OrderStr) > 0 {
		fmt.Fprint(&buf, statement.OrderStr, ", ")
	}
	for i, col := range colNames {
		if i > 0 {
			fmt.Fprint(&buf, ", ")
		}
		statement.dialect.Quoter().QuoteTo(&buf, col)
		fmt.Fprint(&buf, " ASC")
	}
	statement.OrderStr = buf.String()
	return statement
}

// Conds returns condtions
func (statement *Statement) Conds() builder.Cond {
	return statement.cond
}

// SetTable tempororily set table name, the parameter could be a string or a pointer of struct
func (statement *Statement) SetTable(tableNameOrBean interface{}) error {
	v := rValue(tableNameOrBean)
	t := v.Type()
	if t.Kind() == reflect.Struct {
		var err error
		statement.RefTable, err = statement.tagParser.ParseWithCache(v)
		if err != nil {
			return err
		}
	}

	statement.AltTableName = dialects.FullTableName(statement.dialect, statement.tagParser.GetTableMapper(), tableNameOrBean, true)
	return nil
}

// Join The joinOP should be one of INNER, LEFT OUTER, CROSS etc - this will be prepended to JOIN
func (statement *Statement) Join(joinOP string, tablename interface{}, condition string, args ...interface{}) *Statement {
	var buf strings.Builder
	if len(statement.JoinStr) > 0 {
		fmt.Fprintf(&buf, "%v %v JOIN ", statement.JoinStr, joinOP)
	} else {
		fmt.Fprintf(&buf, "%v JOIN ", joinOP)
	}

	switch tp := tablename.(type) {
	case builder.Builder:
		subSQL, subQueryArgs, err := tp.ToSQL()
		if err != nil {
			statement.LastError = err
			return statement
		}

		fields := strings.Split(tp.TableName(), ".")
		aliasName := statement.dialect.Quoter().Trim(fields[len(fields)-1])
		aliasName = schemas.CommonQuoter.Trim(aliasName)

		fmt.Fprintf(&buf, "(%s) %s ON %v", statement.ReplaceQuote(subSQL), aliasName, statement.ReplaceQuote(condition))
		statement.joinArgs = append(statement.joinArgs, subQueryArgs...)
	case *builder.Builder:
		subSQL, subQueryArgs, err := tp.ToSQL()
		if err != nil {
			statement.LastError = err
			return statement
		}

		fields := strings.Split(tp.TableName(), ".")
		aliasName := statement.dialect.Quoter().Trim(fields[len(fields)-1])
		aliasName = schemas.CommonQuoter.Trim(aliasName)

		fmt.Fprintf(&buf, "(%s) %s ON %v", statement.ReplaceQuote(subSQL), aliasName, statement.ReplaceQuote(condition))
		statement.joinArgs = append(statement.joinArgs, subQueryArgs...)
	default:
		tbName := dialects.FullTableName(statement.dialect, statement.tagParser.GetTableMapper(), tablename, true)
		if !utils.IsSubQuery(tbName) {
			var buf strings.Builder
			statement.dialect.Quoter().QuoteTo(&buf, tbName)
			tbName = buf.String()
		}
		fmt.Fprintf(&buf, "%s ON %v", tbName, statement.ReplaceQuote(condition))
	}

	statement.JoinStr = buf.String()
	statement.joinArgs = append(statement.joinArgs, args...)
	return statement
}

// tbNameNoSchema get some table's table name
func (statement *Statement) tbNameNoSchema(table *schemas.Table) string {
	if len(statement.AltTableName) > 0 {
		return statement.AltTableName
	}

	return table.Name
}

// GroupBy generate "Group By keys" statement
func (statement *Statement) GroupBy(keys string) *Statement {
	statement.GroupByStr = statement.ReplaceQuote(keys)
	return statement
}

// Having generate "Having conditions" statement
func (statement *Statement) Having(conditions string) *Statement {
	statement.HavingStr = fmt.Sprintf("HAVING %v", statement.ReplaceQuote(conditions))
	return statement
}

// SetUnscoped always disable struct tag "deleted"
func (statement *Statement) SetUnscoped() *Statement {
	statement.unscoped = true
	return statement
}

// GetUnscoped return true if it's unscoped
func (statement *Statement) GetUnscoped() bool {
	return statement.unscoped
}

func (statement *Statement) genColumnStr() string {
	if statement.RefTable == nil {
		return ""
	}

	var buf strings.Builder
	columns := statement.RefTable.Columns()

	for _, col := range columns {
		if statement.OmitColumnMap.Contain(col.Name) {
			continue
		}

		if len(statement.ColumnMap) > 0 && !statement.ColumnMap.Contain(col.Name) {
			continue
		}

		if col.MapType == schemas.ONLYTODB {
			continue
		}

		if buf.Len() != 0 {
			buf.WriteString(", ")
		}

		if statement.JoinStr != "" {
			if statement.TableAlias != "" {
				buf.WriteString(statement.TableAlias)
			} else {
				buf.WriteString(statement.TableName())
			}

			buf.WriteString(".")
		}

		statement.dialect.Quoter().QuoteTo(&buf, col.Name)
	}

	return buf.String()
}

// GenCreateTableSQL generated create table SQL
func (statement *Statement) GenCreateTableSQL() []string {
	statement.RefTable.StoreEngine = statement.StoreEngine
	statement.RefTable.Charset = statement.Charset
	s, _ := statement.dialect.CreateTableSQL(statement.RefTable, statement.TableName())
	return s
}

// GenIndexSQL generated create index SQL
func (statement *Statement) GenIndexSQL() []string {
	var sqls []string
	tbName := statement.TableName()
	for _, index := range statement.RefTable.Indexes {
		if index.Type == schemas.IndexType {
			sql := statement.dialect.CreateIndexSQL(tbName, index)
			sqls = append(sqls, sql)
		}
	}
	return sqls
}

func uniqueName(tableName, uqeName string) string {
	return fmt.Sprintf("UQE_%v_%v", tableName, uqeName)
}

// GenUniqueSQL generates unique SQL
func (statement *Statement) GenUniqueSQL() []string {
	var sqls []string
	tbName := statement.TableName()
	for _, index := range statement.RefTable.Indexes {
		if index.Type == schemas.UniqueType {
			sql := statement.dialect.CreateIndexSQL(tbName, index)
			sqls = append(sqls, sql)
		}
	}
	return sqls
}

// GenDelIndexSQL generate delete index SQL
func (statement *Statement) GenDelIndexSQL() []string {
	var sqls []string
	tbName := statement.TableName()
	idx := strings.Index(tbName, ".")
	if idx > -1 {
		tbName = tbName[idx+1:]
	}
	for _, index := range statement.RefTable.Indexes {
		sqls = append(sqls, statement.dialect.DropIndexSQL(tbName, index))
	}
	return sqls
}

func (statement *Statement) buildConds2(table *schemas.Table, bean interface{},
	includeVersion bool, includeUpdated bool, includeNil bool,
	includeAutoIncr bool, allUseBool bool, useAllCols bool, unscoped bool,
	mustColumnMap map[string]bool, tableName, aliasName string, addedTableName bool) (builder.Cond, error) {
	var conds []builder.Cond
	for _, col := range table.Columns() {
		if !includeVersion && col.IsVersion {
			continue
		}
		if !includeUpdated && col.IsUpdated {
			continue
		}
		if !includeAutoIncr && col.IsAutoIncrement {
			continue
		}

		if statement.dialect.URI().DBType == schemas.MSSQL && (col.SQLType.Name == schemas.Text ||
			col.SQLType.IsBlob() || col.SQLType.Name == schemas.TimeStampz) {
			continue
		}
		if col.IsJSON {
			continue
		}

		var colName string
		if addedTableName {
			var nm = tableName
			if len(aliasName) > 0 {
				nm = aliasName
			}
			colName = statement.quote(nm) + "." + statement.quote(col.Name)
		} else {
			colName = statement.quote(col.Name)
		}

		fieldValuePtr, err := col.ValueOf(bean)
		if err != nil {
			if !strings.Contains(err.Error(), "is not valid") {
				//engine.logger.Warn(err)
			}
			continue
		} else if fieldValuePtr == nil {
			continue
		}

		if col.IsDeleted && !unscoped { // tag "deleted" is enabled
			conds = append(conds, statement.CondDeleted(col))
		}

		fieldValue := *fieldValuePtr
		if fieldValue.Interface() == nil {
			continue
		}

		fieldType := reflect.TypeOf(fieldValue.Interface())
		requiredField := useAllCols

		if b, ok := getFlagForColumn(mustColumnMap, col); ok {
			if b {
				requiredField = true
			} else {
				continue
			}
		}

		if fieldType.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				if includeNil {
					conds = append(conds, builder.Eq{colName: nil})
				}
				continue
			} else if !fieldValue.IsValid() {
				continue
			} else {
				// dereference ptr type to instance type
				fieldValue = fieldValue.Elem()
				fieldType = reflect.TypeOf(fieldValue.Interface())
				requiredField = true
			}
		}

		var val interface{}
		switch fieldType.Kind() {
		case reflect.Bool:
			if allUseBool || requiredField {
				val = fieldValue.Interface()
			} else {
				// if a bool in a struct, it will not be as a condition because it default is false,
				// please use Where() instead
				continue
			}
		case reflect.String:
			if !requiredField && fieldValue.String() == "" {
				continue
			}
			// for MyString, should convert to string or panic
			if fieldType.String() != reflect.String.String() {
				val = fieldValue.String()
			} else {
				val = fieldValue.Interface()
			}
		case reflect.Int8, reflect.Int16, reflect.Int, reflect.Int32, reflect.Int64:
			if !requiredField && fieldValue.Int() == 0 {
				continue
			}
			val = fieldValue.Interface()
		case reflect.Float32, reflect.Float64:
			if !requiredField && fieldValue.Float() == 0.0 {
				continue
			}
			val = fieldValue.Interface()
		case reflect.Uint8, reflect.Uint16, reflect.Uint, reflect.Uint32, reflect.Uint64:
			if !requiredField && fieldValue.Uint() == 0 {
				continue
			}
			val = fieldValue.Interface()
		case reflect.Struct:
			if fieldType.ConvertibleTo(schemas.TimeType) {
				t := fieldValue.Convert(schemas.TimeType).Interface().(time.Time)
				if !requiredField && (t.IsZero() || !fieldValue.IsValid()) {
					continue
				}
				val = dialects.FormatColumnTime(statement.dialect, statement.defaultTimeZone, col, t)
			} else if _, ok := reflect.New(fieldType).Interface().(convert.Conversion); ok {
				continue
			} else if valNul, ok := fieldValue.Interface().(driver.Valuer); ok {
				val, _ = valNul.Value()
				if val == nil && !requiredField {
					continue
				}
			} else {
				if col.IsJSON {
					if col.SQLType.IsText() {
						bytes, err := json.DefaultJSONHandler.Marshal(fieldValue.Interface())
						if err != nil {
							return nil, err
						}
						val = string(bytes)
					} else if col.SQLType.IsBlob() {
						var bytes []byte
						var err error
						bytes, err = json.DefaultJSONHandler.Marshal(fieldValue.Interface())
						if err != nil {
							return nil, err
						}
						val = bytes
					}
				} else {
					table, err := statement.tagParser.ParseWithCache(fieldValue)
					if err != nil {
						val = fieldValue.Interface()
					} else {
						if len(table.PrimaryKeys) == 1 {
							pkField := reflect.Indirect(fieldValue).FieldByName(table.PKColumns()[0].FieldName)
							// fix non-int pk issues
							//if pkField.Int() != 0 {
							if pkField.IsValid() && !utils.IsZero(pkField.Interface()) {
								val = pkField.Interface()
							} else {
								continue
							}
						} else {
							//TODO: how to handler?
							return nil, fmt.Errorf("not supported %v as %v", fieldValue.Interface(), table.PrimaryKeys)
						}
					}
				}
			}
		case reflect.Array:
			continue
		case reflect.Slice, reflect.Map:
			if fieldValue == reflect.Zero(fieldType) {
				continue
			}
			if fieldValue.IsNil() || !fieldValue.IsValid() || fieldValue.Len() == 0 {
				continue
			}

			if col.SQLType.IsText() {
				bytes, err := json.DefaultJSONHandler.Marshal(fieldValue.Interface())
				if err != nil {
					return nil, err
				}
				val = string(bytes)
			} else if col.SQLType.IsBlob() {
				var bytes []byte
				var err error
				if (fieldType.Kind() == reflect.Array || fieldType.Kind() == reflect.Slice) &&
					fieldType.Elem().Kind() == reflect.Uint8 {
					if fieldValue.Len() > 0 {
						val = fieldValue.Bytes()
					} else {
						continue
					}
				} else {
					bytes, err = json.DefaultJSONHandler.Marshal(fieldValue.Interface())
					if err != nil {
						return nil, err
					}
					val = bytes
				}
			} else {
				continue
			}
		default:
			val = fieldValue.Interface()
		}

		conds = append(conds, builder.Eq{colName: val})
	}

	return builder.And(conds...), nil
}

// BuildConds builds condition
func (statement *Statement) BuildConds(table *schemas.Table, bean interface{}, includeVersion bool, includeUpdated bool, includeNil bool, includeAutoIncr bool, addedTableName bool) (builder.Cond, error) {
	return statement.buildConds2(table, bean, includeVersion, includeUpdated, includeNil, includeAutoIncr, statement.allUseBool, statement.useAllCols,
		statement.unscoped, statement.MustColumnMap, statement.TableName(), statement.TableAlias, addedTableName)
}

func (statement *Statement) mergeConds(bean interface{}) error {
	if !statement.NoAutoCondition && statement.RefTable != nil {
		var addedTableName = (len(statement.JoinStr) > 0)
		autoCond, err := statement.BuildConds(statement.RefTable, bean, true, true, false, true, addedTableName)
		if err != nil {
			return err
		}
		statement.cond = statement.cond.And(autoCond)
	}

	return statement.ProcessIDParam()
}

// GenConds generates conditions
func (statement *Statement) GenConds(bean interface{}) (string, []interface{}, error) {
	if err := statement.mergeConds(bean); err != nil {
		return "", nil, err
	}

	return statement.GenCondSQL(statement.cond)
}

func (statement *Statement) quoteColumnStr(columnStr string) string {
	columns := strings.Split(columnStr, ",")
	return statement.dialect.Quoter().Join(columns, ",")
}

// ConvertSQLOrArgs converts sql or args
func (statement *Statement) ConvertSQLOrArgs(sqlOrArgs ...interface{}) (string, []interface{}, error) {
	sql, args, err := convertSQLOrArgs(sqlOrArgs...)
	if err != nil {
		return "", nil, err
	}
	return statement.ReplaceQuote(sql), args, nil
}

func convertSQLOrArgs(sqlOrArgs ...interface{}) (string, []interface{}, error) {
	switch sqlOrArgs[0].(type) {
	case string:
		return sqlOrArgs[0].(string), sqlOrArgs[1:], nil
	case *builder.Builder:
		return sqlOrArgs[0].(*builder.Builder).ToSQL()
	case builder.Builder:
		bd := sqlOrArgs[0].(builder.Builder)
		return bd.ToSQL()
	}

	return "", nil, ErrUnSupportedType
}

func (statement *Statement) joinColumns(cols []*schemas.Column, includeTableName bool) string {
	var colnames = make([]string, len(cols))
	for i, col := range cols {
		if includeTableName {
			colnames[i] = statement.quote(statement.TableName()) +
				"." + statement.quote(col.Name)
		} else {
			colnames[i] = statement.quote(col.Name)
		}
	}
	return strings.Join(colnames, ", ")
}

// CondDeleted returns the conditions whether a record is soft deleted.
func (statement *Statement) CondDeleted(col *schemas.Column) builder.Cond {
	var colName = statement.quote(col.Name)
	if statement.JoinStr != "" {
		var prefix string
		if statement.TableAlias != "" {
			prefix = statement.TableAlias
		} else {
			prefix = statement.TableName()
		}
		colName = statement.quote(prefix) + "." + statement.quote(col.Name)
	}
	var cond = builder.NewCond()
	if col.SQLType.IsNumeric() {
		cond = builder.Eq{colName: 0}
	} else {
		// FIXME: mssql: The conversion of a nvarchar data type to a datetime data type resulted in an out-of-range value.
		if statement.dialect.URI().DBType != schemas.MSSQL {
			cond = builder.Eq{colName: utils.ZeroTime1}
		}
	}

	if col.Nullable {
		cond = cond.Or(builder.IsNull{colName})
	}

	return cond
}
