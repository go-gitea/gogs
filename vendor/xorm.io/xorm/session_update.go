// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"xorm.io/builder"
	"xorm.io/xorm/caches"
	"xorm.io/xorm/internal/utils"
	"xorm.io/xorm/schemas"
)

// enumerated all errors
var (
	ErrNoColumnsTobeUpdated = errors.New("no columns found to be updated")
)

func (session *Session) cacheUpdate(table *schemas.Table, tableName, sqlStr string, args ...interface{}) error {
	if table == nil ||
		session.tx != nil {
		return ErrCacheFailed
	}

	oldhead, newsql := session.statement.ConvertUpdateSQL(sqlStr)
	if newsql == "" {
		return ErrCacheFailed
	}
	for _, filter := range session.engine.dialect.Filters() {
		newsql = filter.Do(newsql)
	}
	session.engine.logger.Debugf("[cache] new sql: %v, %v", oldhead, newsql)

	var nStart int
	if len(args) > 0 {
		if strings.Index(sqlStr, "?") > -1 {
			nStart = strings.Count(oldhead, "?")
		} else {
			// only for pq, TODO: if any other databse?
			nStart = strings.Count(oldhead, "$")
		}
	}

	cacher := session.engine.GetCacher(tableName)
	session.engine.logger.Debugf("[cache] get cache sql: %v, %v", newsql, args[nStart:])
	ids, err := caches.GetCacheSql(cacher, tableName, newsql, args[nStart:])
	if err != nil {
		rows, err := session.NoCache().queryRows(newsql, args[nStart:]...)
		if err != nil {
			return err
		}
		defer rows.Close()

		ids = make([]schemas.PK, 0)
		for rows.Next() {
			var res = make([]string, len(table.PrimaryKeys))
			err = rows.ScanSlice(&res)
			if err != nil {
				return err
			}
			var pk schemas.PK = make([]interface{}, len(table.PrimaryKeys))
			for i, col := range table.PKColumns() {
				if col.SQLType.IsNumeric() {
					n, err := strconv.ParseInt(res[i], 10, 64)
					if err != nil {
						return err
					}
					pk[i] = n
				} else if col.SQLType.IsText() {
					pk[i] = res[i]
				} else {
					return errors.New("not supported")
				}
			}

			ids = append(ids, pk)
		}
		session.engine.logger.Debugf("[cache] find updated id: %v", ids)
	} /*else {
	    session.engine.LogDebug("[xorm:cacheUpdate] del cached sql:", tableName, newsql, args)
	    cacher.DelIds(tableName, genSqlKey(newsql, args))
	}*/

	for _, id := range ids {
		sid, err := id.ToString()
		if err != nil {
			return err
		}
		if bean := cacher.GetBean(tableName, sid); bean != nil {
			sqls := utils.SplitNNoCase(sqlStr, "where", 2)
			if len(sqls) == 0 || len(sqls) > 2 {
				return ErrCacheFailed
			}

			sqls = utils.SplitNNoCase(sqls[0], "set", 2)
			if len(sqls) != 2 {
				return ErrCacheFailed
			}
			kvs := strings.Split(strings.TrimSpace(sqls[1]), ",")

			for idx, kv := range kvs {
				sps := strings.SplitN(kv, "=", 2)
				sps2 := strings.Split(sps[0], ".")
				colName := sps2[len(sps2)-1]
				colName = session.engine.dialect.Quoter().Trim(colName)
				colName = schemas.CommonQuoter.Trim(colName)

				if col := table.GetColumn(colName); col != nil {
					fieldValue, err := col.ValueOf(bean)
					if err != nil {
						session.engine.logger.Errorf("%v", err)
					} else {
						session.engine.logger.Debugf("[cache] set bean field: %v, %v, %v", bean, colName, fieldValue.Interface())
						if col.IsVersion && session.statement.CheckVersion {
							session.incrVersionFieldValue(fieldValue)
						} else {
							fieldValue.Set(reflect.ValueOf(args[idx]))
						}
					}
				} else {
					session.engine.logger.Errorf("[cache] ERROR: column %v is not table %v's",
						colName, table.Name)
				}
			}

			session.engine.logger.Debugf("[cache] update cache: %v, %v, %v", tableName, id, bean)
			cacher.PutBean(tableName, sid, bean)
		}
	}
	session.engine.logger.Debugf("[cache] clear cached table sql: %v", tableName)
	cacher.ClearIds(tableName)
	return nil
}

// Update records, bean's non-empty fields are updated contents,
// condiBean' non-empty filds are conditions
// CAUTION:
//        1.bool will defaultly be updated content nor conditions
//         You should call UseBool if you have bool to use.
//        2.float32 & float64 may be not inexact as conditions
func (session *Session) Update(bean interface{}, condiBean ...interface{}) (int64, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	defer session.resetStatement()

	if session.statement.LastError != nil {
		return 0, session.statement.LastError
	}

	v := utils.ReflectValue(bean)
	t := v.Type()

	var colNames []string
	var args []interface{}

	// handle before update processors
	for _, closure := range session.beforeClosures {
		closure(bean)
	}
	cleanupProcessorsClosures(&session.beforeClosures) // cleanup after used
	if processor, ok := interface{}(bean).(BeforeUpdateProcessor); ok {
		processor.BeforeUpdate()
	}
	// --

	var err error
	var isMap = t.Kind() == reflect.Map
	var isStruct = t.Kind() == reflect.Struct
	if isStruct {
		if err := session.statement.SetRefBean(bean); err != nil {
			return 0, err
		}

		if len(session.statement.TableName()) <= 0 {
			return 0, ErrTableNotFound
		}

		if session.statement.ColumnStr() == "" {
			colNames, args, err = session.statement.BuildUpdates(v, false, false,
				false, false, true)
		} else {
			colNames, args, err = session.genUpdateColumns(bean)
		}
		if err != nil {
			return 0, err
		}
	} else if isMap {
		colNames = make([]string, 0)
		args = make([]interface{}, 0)
		bValue := reflect.Indirect(reflect.ValueOf(bean))

		for _, v := range bValue.MapKeys() {
			colNames = append(colNames, session.engine.Quote(v.String())+" = ?")
			args = append(args, bValue.MapIndex(v).Interface())
		}
	} else {
		return 0, ErrParamsType
	}

	table := session.statement.RefTable

	if session.statement.UseAutoTime && table != nil && table.Updated != "" {
		if !session.statement.ColumnMap.Contain(table.Updated) &&
			!session.statement.OmitColumnMap.Contain(table.Updated) {
			colNames = append(colNames, session.engine.Quote(table.Updated)+" = ?")
			col := table.UpdatedColumn()
			val, t := session.engine.nowTime(col)
			if session.engine.dialect.URI().DBType == schemas.ORACLE {
				args = append(args, t)
			} else {
				args = append(args, val)
			}

			var colName = col.Name
			if isStruct {
				session.afterClosures = append(session.afterClosures, func(bean interface{}) {
					col := table.GetColumn(colName)
					setColumnTime(bean, col, t)
				})
			}
		}
	}

	// for update action to like "column = column + ?"
	incColumns := session.statement.IncrColumns
	for _, expr := range incColumns {
		colNames = append(colNames, session.engine.Quote(expr.ColName)+" = "+session.engine.Quote(expr.ColName)+" + ?")
		args = append(args, expr.Arg)
	}
	// for update action to like "column = column - ?"
	decColumns := session.statement.DecrColumns
	for _, expr := range decColumns {
		colNames = append(colNames, session.engine.Quote(expr.ColName)+" = "+session.engine.Quote(expr.ColName)+" - ?")
		args = append(args, expr.Arg)
	}
	// for update action to like "column = expression"
	exprColumns := session.statement.ExprColumns
	for _, expr := range exprColumns {
		switch tp := expr.Arg.(type) {
		case string:
			if len(tp) == 0 {
				tp = "''"
			}
			colNames = append(colNames, session.engine.Quote(expr.ColName)+"="+tp)
		case *builder.Builder:
			subQuery, subArgs, err := session.statement.GenCondSQL(tp)
			if err != nil {
				return 0, err
			}
			colNames = append(colNames, session.engine.Quote(expr.ColName)+"=("+subQuery+")")
			args = append(args, subArgs...)
		default:
			colNames = append(colNames, session.engine.Quote(expr.ColName)+"=?")
			args = append(args, expr.Arg)
		}
	}

	if err = session.statement.ProcessIDParam(); err != nil {
		return 0, err
	}

	var autoCond builder.Cond
	if !session.statement.NoAutoCondition {
		condBeanIsStruct := false
		if len(condiBean) > 0 {
			if c, ok := condiBean[0].(map[string]interface{}); ok {
				autoCond = builder.Eq(c)
			} else {
				ct := reflect.TypeOf(condiBean[0])
				k := ct.Kind()
				if k == reflect.Ptr {
					k = ct.Elem().Kind()
				}
				if k == reflect.Struct {
					condTable, err := session.engine.TableInfo(condiBean[0])
					if err != nil {
						return 0, err
					}

					autoCond, err = session.statement.BuildConds(condTable, condiBean[0], true, true, false, true, false)
					if err != nil {
						return 0, err
					}
					condBeanIsStruct = true
				} else {
					return 0, ErrConditionType
				}
			}
		}

		if !condBeanIsStruct && table != nil {
			if col := table.DeletedColumn(); col != nil && !session.statement.GetUnscoped() { // tag "deleted" is enabled
				autoCond1 := session.statement.CondDeleted(col)

				if autoCond == nil {
					autoCond = autoCond1
				} else {
					autoCond = autoCond.And(autoCond1)
				}
			}
		}
	}

	st := session.statement

	var (
		sqlStr   string
		condArgs []interface{}
		condSQL  string
		cond     = session.statement.Conds().And(autoCond)

		doIncVer = isStruct && (table != nil && table.Version != "" && session.statement.CheckVersion)
		verValue *reflect.Value
	)
	if doIncVer {
		verValue, err = table.VersionColumn().ValueOf(bean)
		if err != nil {
			return 0, err
		}

		if verValue != nil {
			cond = cond.And(builder.Eq{session.engine.Quote(table.Version): verValue.Interface()})
			colNames = append(colNames, session.engine.Quote(table.Version)+" = "+session.engine.Quote(table.Version)+" + 1")
		}
	}

	if len(colNames) <= 0 {
		return 0, ErrNoColumnsTobeUpdated
	}

	condSQL, condArgs, err = session.statement.GenCondSQL(cond)
	if err != nil {
		return 0, err
	}

	if len(condSQL) > 0 {
		condSQL = "WHERE " + condSQL
	}

	if st.OrderStr != "" {
		condSQL = condSQL + fmt.Sprintf(" ORDER BY %v", st.OrderStr)
	}

	var tableName = session.statement.TableName()
	// TODO: Oracle support needed
	var top string
	if st.LimitN != nil {
		limitValue := *st.LimitN
		switch session.engine.dialect.URI().DBType {
		case schemas.MYSQL:
			condSQL = condSQL + fmt.Sprintf(" LIMIT %d", limitValue)
		case schemas.SQLITE:
			tempCondSQL := condSQL + fmt.Sprintf(" LIMIT %d", limitValue)
			cond = cond.And(builder.Expr(fmt.Sprintf("rowid IN (SELECT rowid FROM %v %v)",
				session.engine.Quote(tableName), tempCondSQL), condArgs...))
			condSQL, condArgs, err = session.statement.GenCondSQL(cond)
			if err != nil {
				return 0, err
			}
			if len(condSQL) > 0 {
				condSQL = "WHERE " + condSQL
			}
		case schemas.POSTGRES:
			tempCondSQL := condSQL + fmt.Sprintf(" LIMIT %d", limitValue)
			cond = cond.And(builder.Expr(fmt.Sprintf("CTID IN (SELECT CTID FROM %v %v)",
				session.engine.Quote(tableName), tempCondSQL), condArgs...))
			condSQL, condArgs, err = session.statement.GenCondSQL(cond)
			if err != nil {
				return 0, err
			}

			if len(condSQL) > 0 {
				condSQL = "WHERE " + condSQL
			}
		case schemas.MSSQL:
			if st.OrderStr != "" && table != nil && len(table.PrimaryKeys) == 1 {
				cond = builder.Expr(fmt.Sprintf("%s IN (SELECT TOP (%d) %s FROM %v%v)",
					table.PrimaryKeys[0], limitValue, table.PrimaryKeys[0],
					session.engine.Quote(tableName), condSQL), condArgs...)

				condSQL, condArgs, err = session.statement.GenCondSQL(cond)
				if err != nil {
					return 0, err
				}
				if len(condSQL) > 0 {
					condSQL = "WHERE " + condSQL
				}
			} else {
				top = fmt.Sprintf("TOP (%d) ", limitValue)
			}
		}
	}

	var tableAlias = session.engine.Quote(tableName)
	var fromSQL string
	if session.statement.TableAlias != "" {
		switch session.engine.dialect.URI().DBType {
		case schemas.MSSQL:
			fromSQL = fmt.Sprintf("FROM %s %s ", tableAlias, session.statement.TableAlias)
			tableAlias = session.statement.TableAlias
		default:
			tableAlias = fmt.Sprintf("%s AS %s", tableAlias, session.statement.TableAlias)
		}
	}

	sqlStr = fmt.Sprintf("UPDATE %v%v SET %v %v%v",
		top,
		tableAlias,
		strings.Join(colNames, ", "),
		fromSQL,
		condSQL)

	res, err := session.exec(sqlStr, append(args, condArgs...)...)
	if err != nil {
		return 0, err
	} else if doIncVer {
		if verValue != nil && verValue.IsValid() && verValue.CanSet() {
			session.incrVersionFieldValue(verValue)
		}
	}

	if cacher := session.engine.GetCacher(tableName); cacher != nil && session.statement.UseCache {
		// session.cacheUpdate(table, tableName, sqlStr, args...)
		session.engine.logger.Debugf("[cache] clear table: %v", tableName)
		cacher.ClearIds(tableName)
		cacher.ClearBeans(tableName)
	}

	// handle after update processors
	if session.isAutoCommit {
		for _, closure := range session.afterClosures {
			closure(bean)
		}
		if processor, ok := interface{}(bean).(AfterUpdateProcessor); ok {
			session.engine.logger.Debugf("[event] %v has after update processor", tableName)
			processor.AfterUpdate()
		}
	} else {
		lenAfterClosures := len(session.afterClosures)
		if lenAfterClosures > 0 {
			if value, has := session.afterUpdateBeans[bean]; has && value != nil {
				*value = append(*value, session.afterClosures...)
			} else {
				afterClosures := make([]func(interface{}), lenAfterClosures)
				copy(afterClosures, session.afterClosures)
				// FIXME: if bean is a map type, it will panic because map cannot be as map key
				session.afterUpdateBeans[bean] = &afterClosures
			}
		} else {
			if _, ok := interface{}(bean).(AfterUpdateProcessor); ok {
				session.afterUpdateBeans[bean] = nil
			}
		}
	}
	cleanupProcessorsClosures(&session.afterClosures) // cleanup after used
	// --

	return res.RowsAffected()
}

func (session *Session) genUpdateColumns(bean interface{}) ([]string, []interface{}, error) {
	table := session.statement.RefTable
	colNames := make([]string, 0, len(table.ColumnsSeq()))
	args := make([]interface{}, 0, len(table.ColumnsSeq()))

	for _, col := range table.Columns() {
		if !col.IsVersion && !col.IsCreated && !col.IsUpdated {
			if session.statement.OmitColumnMap.Contain(col.Name) {
				continue
			}
		}
		if col.MapType == schemas.ONLYFROMDB {
			continue
		}

		fieldValuePtr, err := col.ValueOf(bean)
		if err != nil {
			return nil, nil, err
		}
		fieldValue := *fieldValuePtr

		if col.IsAutoIncrement && utils.IsValueZero(fieldValue) {
			continue
		}

		if (col.IsDeleted && !session.statement.GetUnscoped()) || col.IsCreated {
			continue
		}

		// if only update specify columns
		if len(session.statement.ColumnMap) > 0 && !session.statement.ColumnMap.Contain(col.Name) {
			continue
		}

		if session.statement.IncrColumns.IsColExist(col.Name) {
			continue
		} else if session.statement.DecrColumns.IsColExist(col.Name) {
			continue
		} else if session.statement.ExprColumns.IsColExist(col.Name) {
			continue
		}

		// !evalphobia! set fieldValue as nil when column is nullable and zero-value
		if _, ok := getFlagForColumn(session.statement.NullableMap, col); ok {
			if col.Nullable && utils.IsValueZero(fieldValue) {
				var nilValue *int
				fieldValue = reflect.ValueOf(nilValue)
			}
		}

		if col.IsUpdated && session.statement.UseAutoTime /*&& isZero(fieldValue.Interface())*/ {
			// if time is non-empty, then set to auto time
			val, t := session.engine.nowTime(col)
			args = append(args, val)

			var colName = col.Name
			session.afterClosures = append(session.afterClosures, func(bean interface{}) {
				col := table.GetColumn(colName)
				setColumnTime(bean, col, t)
			})
		} else if col.IsVersion && session.statement.CheckVersion {
			args = append(args, 1)
		} else {
			arg, err := session.statement.Value2Interface(col, fieldValue)
			if err != nil {
				return colNames, args, err
			}
			args = append(args, arg)
		}

		colNames = append(colNames, session.engine.Quote(col.Name)+" = ?")
	}
	return colNames, args, nil
}
