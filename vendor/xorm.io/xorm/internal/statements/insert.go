// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"fmt"
	"strings"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

func (statement *Statement) writeInsertOutput(buf *strings.Builder, table *schemas.Table) error {
	if statement.dialect.URI().DBType == schemas.MSSQL && len(table.AutoIncrement) > 0 {
		if _, err := buf.WriteString(" OUTPUT Inserted."); err != nil {
			return err
		}
		if err := statement.dialect.Quoter().QuoteTo(buf, table.AutoIncrement); err != nil {
			return err
		}
	}
	return nil
}

// GenInsertSQL generates insert beans SQL
func (statement *Statement) GenInsertSQL(colNames []string, args []interface{}) (string, []interface{}, error) {
	var (
		buf       = builder.NewWriter()
		exprs     = statement.ExprColumns
		table     = statement.RefTable
		tableName = statement.TableName()
	)

	if _, err := buf.WriteString("INSERT INTO "); err != nil {
		return "", nil, err
	}

	if err := statement.dialect.Quoter().QuoteTo(buf.Builder, tableName); err != nil {
		return "", nil, err
	}

	if len(colNames) <= 0 {
		if statement.dialect.URI().DBType == schemas.MYSQL {
			if _, err := buf.WriteString(" VALUES ()"); err != nil {
				return "", nil, err
			}
		} else {
			if err := statement.writeInsertOutput(buf.Builder, table); err != nil {
				return "", nil, err
			}
			if _, err := buf.WriteString(" DEFAULT VALUES"); err != nil {
				return "", nil, err
			}
		}
	} else {
		if _, err := buf.WriteString(" ("); err != nil {
			return "", nil, err
		}

		if err := statement.dialect.Quoter().JoinWrite(buf.Builder, append(colNames, exprs.ColNames()...), ","); err != nil {
			return "", nil, err
		}

		if _, err := buf.WriteString(")"); err != nil {
			return "", nil, err
		}
		if err := statement.writeInsertOutput(buf.Builder, table); err != nil {
			return "", nil, err
		}

		if statement.Conds().IsValid() {
			if _, err := buf.WriteString(" SELECT "); err != nil {
				return "", nil, err
			}

			if err := statement.WriteArgs(buf, args); err != nil {
				return "", nil, err
			}

			if len(exprs) > 0 {
				if _, err := buf.WriteString(","); err != nil {
					return "", nil, err
				}
			}
			if err := exprs.WriteArgs(buf); err != nil {
				return "", nil, err
			}

			if _, err := buf.WriteString(" FROM "); err != nil {
				return "", nil, err
			}

			if err := statement.dialect.Quoter().QuoteTo(buf.Builder, tableName); err != nil {
				return "", nil, err
			}

			if _, err := buf.WriteString(" WHERE "); err != nil {
				return "", nil, err
			}

			if err := statement.Conds().WriteTo(buf); err != nil {
				return "", nil, err
			}
		} else {
			if _, err := buf.WriteString(" VALUES ("); err != nil {
				return "", nil, err
			}

			if err := statement.WriteArgs(buf, args); err != nil {
				return "", nil, err
			}

			if len(exprs) > 0 {
				if _, err := buf.WriteString(","); err != nil {
					return "", nil, err
				}
			}

			if err := exprs.WriteArgs(buf); err != nil {
				return "", nil, err
			}

			if _, err := buf.WriteString(")"); err != nil {
				return "", nil, err
			}
		}
	}

	if len(table.AutoIncrement) > 0 && statement.dialect.URI().DBType == schemas.POSTGRES {
		if _, err := buf.WriteString(" RETURNING "); err != nil {
			return "", nil, err
		}
		if err := statement.dialect.Quoter().QuoteTo(buf.Builder, table.AutoIncrement); err != nil {
			return "", nil, err
		}
	}

	return buf.String(), buf.Args(), nil
}

// GenInsertMapSQL generates insert map SQL
func (statement *Statement) GenInsertMapSQL(columns []string, args []interface{}) (string, []interface{}, error) {
	var (
		buf       = builder.NewWriter()
		exprs     = statement.ExprColumns
		tableName = statement.TableName()
	)

	if _, err := buf.WriteString(fmt.Sprintf("INSERT INTO %s (", statement.quote(tableName))); err != nil {
		return "", nil, err
	}

	if err := statement.dialect.Quoter().JoinWrite(buf.Builder, append(columns, exprs.ColNames()...), ","); err != nil {
		return "", nil, err
	}

	// if insert where
	if statement.Conds().IsValid() {
		if _, err := buf.WriteString(") SELECT "); err != nil {
			return "", nil, err
		}

		if err := statement.WriteArgs(buf, args); err != nil {
			return "", nil, err
		}

		if len(exprs) > 0 {
			if _, err := buf.WriteString(","); err != nil {
				return "", nil, err
			}
			if err := exprs.WriteArgs(buf); err != nil {
				return "", nil, err
			}
		}

		if _, err := buf.WriteString(fmt.Sprintf(" FROM %s WHERE ", statement.quote(tableName))); err != nil {
			return "", nil, err
		}

		if err := statement.Conds().WriteTo(buf); err != nil {
			return "", nil, err
		}
	} else {
		if _, err := buf.WriteString(") VALUES ("); err != nil {
			return "", nil, err
		}
		if err := statement.WriteArgs(buf, args); err != nil {
			return "", nil, err
		}

		if len(exprs) > 0 {
			if _, err := buf.WriteString(","); err != nil {
				return "", nil, err
			}
			if err := exprs.WriteArgs(buf); err != nil {
				return "", nil, err
			}
		}
		if _, err := buf.WriteString(")"); err != nil {
			return "", nil, err
		}
	}

	return buf.String(), buf.Args(), nil
}
