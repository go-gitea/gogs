// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"xorm.io/xorm/core"
	"xorm.io/xorm/schemas"
)

var (
	mssqlReservedWords = map[string]bool{
		"ADD":                            true,
		"EXTERNAL":                       true,
		"PROCEDURE":                      true,
		"ALL":                            true,
		"FETCH":                          true,
		"PUBLIC":                         true,
		"ALTER":                          true,
		"FILE":                           true,
		"RAISERROR":                      true,
		"AND":                            true,
		"FILLFACTOR":                     true,
		"READ":                           true,
		"ANY":                            true,
		"FOR":                            true,
		"READTEXT":                       true,
		"AS":                             true,
		"FOREIGN":                        true,
		"RECONFIGURE":                    true,
		"ASC":                            true,
		"FREETEXT":                       true,
		"REFERENCES":                     true,
		"AUTHORIZATION":                  true,
		"FREETEXTTABLE":                  true,
		"REPLICATION":                    true,
		"BACKUP":                         true,
		"FROM":                           true,
		"RESTORE":                        true,
		"BEGIN":                          true,
		"FULL":                           true,
		"RESTRICT":                       true,
		"BETWEEN":                        true,
		"FUNCTION":                       true,
		"RETURN":                         true,
		"BREAK":                          true,
		"GOTO":                           true,
		"REVERT":                         true,
		"BROWSE":                         true,
		"GRANT":                          true,
		"REVOKE":                         true,
		"BULK":                           true,
		"GROUP":                          true,
		"RIGHT":                          true,
		"BY":                             true,
		"HAVING":                         true,
		"ROLLBACK":                       true,
		"CASCADE":                        true,
		"HOLDLOCK":                       true,
		"ROWCOUNT":                       true,
		"CASE":                           true,
		"IDENTITY":                       true,
		"ROWGUIDCOL":                     true,
		"CHECK":                          true,
		"IDENTITY_INSERT":                true,
		"RULE":                           true,
		"CHECKPOINT":                     true,
		"IDENTITYCOL":                    true,
		"SAVE":                           true,
		"CLOSE":                          true,
		"IF":                             true,
		"SCHEMA":                         true,
		"CLUSTERED":                      true,
		"IN":                             true,
		"SECURITYAUDIT":                  true,
		"COALESCE":                       true,
		"INDEX":                          true,
		"SELECT":                         true,
		"COLLATE":                        true,
		"INNER":                          true,
		"SEMANTICKEYPHRASETABLE":         true,
		"COLUMN":                         true,
		"INSERT":                         true,
		"SEMANTICSIMILARITYDETAILSTABLE": true,
		"COMMIT":                         true,
		"INTERSECT":                      true,
		"SEMANTICSIMILARITYTABLE":        true,
		"COMPUTE":                        true,
		"INTO":                           true,
		"SESSION_USER":                   true,
		"CONSTRAINT":                     true,
		"IS":                             true,
		"SET":                            true,
		"CONTAINS":                       true,
		"JOIN":                           true,
		"SETUSER":                        true,
		"CONTAINSTABLE":                  true,
		"KEY":                            true,
		"SHUTDOWN":                       true,
		"CONTINUE":                       true,
		"KILL":                           true,
		"SOME":                           true,
		"CONVERT":                        true,
		"LEFT":                           true,
		"STATISTICS":                     true,
		"CREATE":                         true,
		"LIKE":                           true,
		"SYSTEM_USER":                    true,
		"CROSS":                          true,
		"LINENO":                         true,
		"TABLE":                          true,
		"CURRENT":                        true,
		"LOAD":                           true,
		"TABLESAMPLE":                    true,
		"CURRENT_DATE":                   true,
		"MERGE":                          true,
		"TEXTSIZE":                       true,
		"CURRENT_TIME":                   true,
		"NATIONAL":                       true,
		"THEN":                           true,
		"CURRENT_TIMESTAMP":              true,
		"NOCHECK":                        true,
		"TO":                             true,
		"CURRENT_USER":                   true,
		"NONCLUSTERED":                   true,
		"TOP":                            true,
		"CURSOR":                         true,
		"NOT":                            true,
		"TRAN":                           true,
		"DATABASE":                       true,
		"NULL":                           true,
		"TRANSACTION":                    true,
		"DBCC":                           true,
		"NULLIF":                         true,
		"TRIGGER":                        true,
		"DEALLOCATE":                     true,
		"OF":                             true,
		"TRUNCATE":                       true,
		"DECLARE":                        true,
		"OFF":                            true,
		"TRY_CONVERT":                    true,
		"DEFAULT":                        true,
		"OFFSETS":                        true,
		"TSEQUAL":                        true,
		"DELETE":                         true,
		"ON":                             true,
		"UNION":                          true,
		"DENY":                           true,
		"OPEN":                           true,
		"UNIQUE":                         true,
		"DESC":                           true,
		"OPENDATASOURCE":                 true,
		"UNPIVOT":                        true,
		"DISK":                           true,
		"OPENQUERY":                      true,
		"UPDATE":                         true,
		"DISTINCT":                       true,
		"OPENROWSET":                     true,
		"UPDATETEXT":                     true,
		"DISTRIBUTED":                    true,
		"OPENXML":                        true,
		"USE":                            true,
		"DOUBLE":                         true,
		"OPTION":                         true,
		"USER":                           true,
		"DROP":                           true,
		"OR":                             true,
		"VALUES":                         true,
		"DUMP":                           true,
		"ORDER":                          true,
		"VARYING":                        true,
		"ELSE":                           true,
		"OUTER":                          true,
		"VIEW":                           true,
		"END":                            true,
		"OVER":                           true,
		"WAITFOR":                        true,
		"ERRLVL":                         true,
		"PERCENT":                        true,
		"WHEN":                           true,
		"ESCAPE":                         true,
		"PIVOT":                          true,
		"WHERE":                          true,
		"EXCEPT":                         true,
		"PLAN":                           true,
		"WHILE":                          true,
		"EXEC":                           true,
		"PRECISION":                      true,
		"WITH":                           true,
		"EXECUTE":                        true,
		"PRIMARY":                        true,
		"WITHIN":                         true,
		"EXISTS":                         true,
		"PRINT":                          true,
		"WRITETEXT":                      true,
		"EXIT":                           true,
		"PROC":                           true,
	}

	mssqlQuoter = schemas.Quoter{
		Prefix:     '[',
		Suffix:     ']',
		IsReserved: schemas.AlwaysReserve,
	}
)

type mssql struct {
	Base
	defaultVarchar string
	defaultChar    string
}

func (db *mssql) Init(uri *URI) error {
	db.quoter = mssqlQuoter
	db.defaultChar = "CHAR"
	db.defaultVarchar = "VARCHAR"
	return db.Base.Init(db, uri)
}

func (db *mssql) SetParams(params map[string]string) {
	defaultVarchar, ok := params["DEFAULT_VARCHAR"]
	if ok {
		var t = strings.ToUpper(defaultVarchar)
		switch t {
		case "NVARCHAR", "VARCHAR":
			db.defaultVarchar = t
		default:
			db.defaultVarchar = "VARCHAR"
		}
	} else {
		db.defaultVarchar = "VARCHAR"
	}

	defaultChar, ok := params["DEFAULT_CHAR"]
	if ok {
		var t = strings.ToUpper(defaultChar)
		switch t {
		case "NCHAR", "CHAR":
			db.defaultChar = t
		default:
			db.defaultChar = "CHAR"
		}
	} else {
		db.defaultChar = "CHAR"
	}
}

func (db *mssql) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx,
		"SELECT SERVERPROPERTY('productversion'), SERVERPROPERTY ('productlevel') AS ProductLevel, SERVERPROPERTY ('edition') AS ProductEdition")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var version, level, edition string
	if !rows.Next() {
		return nil, errors.New("unknow version")
	}

	if err := rows.Scan(&version, &level, &edition); err != nil {
		return nil, err
	}

	// MSSQL: Microsoft SQL Server 2017 (RTM-CU13) (KB4466404) - 14.0.3048.4 (X64) Nov 30 2018 12:57:58 Copyright (C) 2017 Microsoft Corporation Developer Edition (64-bit) on Linux (Ubuntu 16.04.5 LTS)
	return &schemas.Version{
		Number:  version,
		Level:   level,
		Edition: edition,
	}, nil
}

func (db *mssql) SQLType(c *schemas.Column) string {
	var res string
	switch t := c.SQLType.Name; t {
	case schemas.Bool:
		res = schemas.Bit
		if strings.EqualFold(c.Default, "true") {
			c.Default = "1"
		} else if strings.EqualFold(c.Default, "false") {
			c.Default = "0"
		}
		return res
	case schemas.Serial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = schemas.Int
	case schemas.BigSerial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = schemas.BigInt
	case schemas.Bytea, schemas.Blob, schemas.Binary, schemas.TinyBlob, schemas.MediumBlob, schemas.LongBlob:
		res = schemas.VarBinary
		if c.Length == 0 {
			c.Length = 50
		}
	case schemas.TimeStamp:
		res = schemas.DateTime
	case schemas.TimeStampz:
		res = "DATETIMEOFFSET"
		c.Length = 7
	case schemas.MediumInt:
		res = schemas.Int
	case schemas.Text, schemas.MediumText, schemas.TinyText, schemas.LongText, schemas.Json:
		res = db.defaultVarchar + "(MAX)"
	case schemas.Double:
		res = schemas.Real
	case schemas.Uuid:
		res = schemas.Varchar
		c.Length = 40
	case schemas.TinyInt:
		res = schemas.TinyInt
		c.Length = 0
	case schemas.BigInt, schemas.UnsignedBigInt, schemas.UnsignedInt:
		res = schemas.BigInt
		c.Length = 0
	case schemas.NVarchar:
		res = t
		if c.Length == -1 {
			res += "(MAX)"
		}
	case schemas.Varchar:
		res = db.defaultVarchar
		if c.Length == -1 {
			res += "(MAX)"
		}
	case schemas.Char:
		res = db.defaultChar
		if c.Length == -1 {
			res += "(MAX)"
		}
	case schemas.NChar:
		res = t
		if c.Length == -1 {
			res += "(MAX)"
		}
	default:
		res = t
	}

	if res == schemas.Int || res == schemas.Bit || res == schemas.DateTime {
		return res
	}

	hasLen1 := (c.Length > 0)
	hasLen2 := (c.Length2 > 0)

	if hasLen2 {
		res += "(" + strconv.Itoa(c.Length) + "," + strconv.Itoa(c.Length2) + ")"
	} else if hasLen1 {
		res += "(" + strconv.Itoa(c.Length) + ")"
	}
	return res
}

func (db *mssql) IsReserved(name string) bool {
	_, ok := mssqlReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *mssql) SetQuotePolicy(quotePolicy QuotePolicy) {
	switch quotePolicy {
	case QuotePolicyNone:
		var q = mssqlQuoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case QuotePolicyReserved:
		var q = mssqlQuoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = mssqlQuoter
	}
}

func (db *mssql) AutoIncrStr() string {
	return "IDENTITY"
}

func (db *mssql) DropTableSQL(tableName string) (string, bool) {
	return fmt.Sprintf("IF EXISTS (SELECT * FROM sysobjects WHERE id = "+
		"object_id(N'%s') and OBJECTPROPERTY(id, N'IsUserTable') = 1) "+
		"DROP TABLE \"%s\"", tableName, tableName), true
}

func (db *mssql) ModifyColumnSQL(tableName string, col *schemas.Column) string {
	s, _ := ColumnString(db.dialect, col, false)
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s", tableName, s)
}

func (db *mssql) IndexCheckSQL(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{idxName}
	sql := "select name from sysindexes where id=object_id('" + tableName + "') and name=?"
	return sql, args
}

func (db *mssql) IsColumnExist(queryer core.Queryer, ctx context.Context, tableName, colName string) (bool, error) {
	query := `SELECT "COLUMN_NAME" FROM "INFORMATION_SCHEMA"."COLUMNS" WHERE "TABLE_NAME" = ? AND "COLUMN_NAME" = ?`

	return db.HasRecords(queryer, ctx, query, tableName, colName)
}

func (db *mssql) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	sql := "select * from sysobjects where id = object_id(N'" + tableName + "') and OBJECTPROPERTY(id, N'IsUserTable') = 1"
	return db.HasRecords(queryer, ctx, sql)
}

func (db *mssql) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	args := []interface{}{}
	s := `select a.name as name, b.name as ctype,a.max_length,a.precision,a.scale,a.is_nullable as nullable,
		  "default_is_null" = (CASE WHEN c.text is null THEN 1 ELSE 0 END),
	      replace(replace(isnull(c.text,''),'(',''),')','') as vdefault,
		  ISNULL(p.is_primary_key, 0), a.is_identity as is_identity
          from sys.columns a 
		  left join sys.types b on a.user_type_id=b.user_type_id
          left join sys.syscomments c on a.default_object_id=c.id
		  LEFT OUTER JOIN (SELECT i.object_id, ic.column_id, i.is_primary_key
			FROM sys.indexes i
		  LEFT JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
			WHERE i.is_primary_key = 1
		) as p on p.object_id = a.object_id AND p.column_id = a.column_id
          where a.object_id=object_id('` + tableName + `')`

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*schemas.Column)
	colSeq := make([]string, 0)
	for rows.Next() {
		var name, ctype, vdefault string
		var maxLen, precision, scale int
		var nullable, isPK, defaultIsNull, isIncrement bool
		err = rows.Scan(&name, &ctype, &maxLen, &precision, &scale, &nullable, &defaultIsNull, &vdefault, &isPK, &isIncrement)
		if err != nil {
			return nil, nil, err
		}

		col := new(schemas.Column)
		col.Indexes = make(map[string]int)
		col.Name = strings.Trim(name, "` ")
		col.Nullable = nullable
		col.DefaultIsEmpty = defaultIsNull
		if !defaultIsNull {
			col.Default = vdefault
		}
		col.IsPrimaryKey = isPK
		col.IsAutoIncrement = isIncrement
		ct := strings.ToUpper(ctype)
		if ct == "DECIMAL" {
			col.Length = precision
			col.Length2 = scale
		} else {
			col.Length = maxLen
		}
		switch ct {
		case "DATETIMEOFFSET":
			col.SQLType = schemas.SQLType{Name: schemas.TimeStampz, DefaultLength: 0, DefaultLength2: 0}
		case "NVARCHAR":
			col.SQLType = schemas.SQLType{Name: schemas.NVarchar, DefaultLength: 0, DefaultLength2: 0}
			if col.Length > 0 {
				col.Length /= 2
				col.Length2 /= 2
			}
		case "IMAGE":
			col.SQLType = schemas.SQLType{Name: schemas.VarBinary, DefaultLength: 0, DefaultLength2: 0}
		case "NCHAR":
			if col.Length > 0 {
				col.Length /= 2
				col.Length2 /= 2
			}
			fallthrough
		default:
			if _, ok := schemas.SqlTypes[ct]; ok {
				col.SQLType = schemas.SQLType{Name: ct, DefaultLength: 0, DefaultLength2: 0}
			} else {
				return nil, nil, fmt.Errorf("Unknown colType %v for %v - %v", ct, tableName, col.Name)
			}
		}

		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	return colSeq, cols, nil
}

func (db *mssql) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) {
	args := []interface{}{}
	s := `select name from sysobjects where xtype ='U'`

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		table := schemas.NewEmptyTable()
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		table.Name = strings.Trim(name, "` ")
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *mssql) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	args := []interface{}{tableName}
	s := `SELECT
IXS.NAME                    AS  [INDEX_NAME],
C.NAME                      AS  [COLUMN_NAME],
IXS.is_unique AS [IS_UNIQUE]
FROM SYS.INDEXES IXS
INNER JOIN SYS.INDEX_COLUMNS   IXCS
ON IXS.OBJECT_ID=IXCS.OBJECT_ID  AND IXS.INDEX_ID = IXCS.INDEX_ID
INNER   JOIN SYS.COLUMNS C  ON IXS.OBJECT_ID=C.OBJECT_ID
AND IXCS.COLUMN_ID=C.COLUMN_ID
WHERE IXS.TYPE_DESC='NONCLUSTERED' and OBJECT_NAME(IXS.OBJECT_ID) =?
`

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index, 0)
	for rows.Next() {
		var indexType int
		var indexName, colName, isUnique string

		err = rows.Scan(&indexName, &colName, &isUnique)
		if err != nil {
			return nil, err
		}

		i, err := strconv.ParseBool(isUnique)
		if err != nil {
			return nil, err
		}

		if i {
			indexType = schemas.UniqueType
		} else {
			indexType = schemas.IndexType
		}

		colName = strings.Trim(colName, "` ")
		var isRegular bool
		if (strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName)) && len(indexName) > (5+len(tableName)) {
			indexName = indexName[5+len(tableName):]
			isRegular = true
		}

		var index *schemas.Index
		var ok bool
		if index, ok = indexes[indexName]; !ok {
			index = new(schemas.Index)
			index.Type = indexType
			index.Name = indexName
			index.IsRegular = isRegular
			indexes[indexName] = index
		}
		index.AddColumn(colName)
	}
	return indexes, nil
}

func (db *mssql) CreateTableSQL(table *schemas.Table, tableName string) ([]string, bool) {
	var sql string
	if tableName == "" {
		tableName = table.Name
	}

	sql = "IF NOT EXISTS (SELECT [name] FROM sys.tables WHERE [name] = '" + tableName + "' ) CREATE TABLE "

	sql += db.Quoter().Quote(tableName) + " ("

	pkList := table.PrimaryKeys

	for _, colName := range table.ColumnsSeq() {
		col := table.GetColumn(colName)
		s, _ := ColumnString(db, col, col.IsPrimaryKey && len(pkList) == 1)
		sql += s
		sql = strings.TrimSpace(sql)
		sql += ", "
	}

	if len(pkList) > 1 {
		sql += "PRIMARY KEY ( "
		sql += strings.Join(pkList, ",")
		sql += " ), "
	}

	sql = sql[:len(sql)-2] + ")"
	sql += ";"
	return []string{sql}, true
}

func (db *mssql) ForUpdateSQL(query string) string {
	return query
}

func (db *mssql) Filters() []Filter {
	return []Filter{}
}

type odbcDriver struct {
}

func (p *odbcDriver) Parse(driverName, dataSourceName string) (*URI, error) {
	var dbName string

	if strings.HasPrefix(dataSourceName, "sqlserver://") {
		u, err := url.Parse(dataSourceName)
		if err != nil {
			return nil, err
		}
		dbName = u.Query().Get("database")
	} else {
		kv := strings.Split(dataSourceName, ";")
		for _, c := range kv {
			vv := strings.Split(strings.TrimSpace(c), "=")
			if len(vv) == 2 {
				switch strings.ToLower(vv[0]) {
				case "database":
					dbName = vv[1]
				}
			}
		}
	}
	if dbName == "" {
		return nil, errors.New("no db name provided")
	}
	return &URI{DBName: dbName, DBType: schemas.MSSQL}, nil
}
