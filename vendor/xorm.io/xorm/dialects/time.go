// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"time"

	"xorm.io/xorm/schemas"
)

// FormatTime format time as column type
func FormatTime(dialect Dialect, sqlTypeName string, t time.Time) (v interface{}) {
	switch sqlTypeName {
	case schemas.Time:
		s := t.Format("2006-01-02 15:04:05") // time.RFC3339
		v = s[11:19]
	case schemas.Date:
		v = t.Format("2006-01-02")
	case schemas.DateTime, schemas.TimeStamp, schemas.Varchar: // !DarthPestilane! format time when sqlTypeName is schemas.Varchar.
		if dialect.URI().DBType == schemas.ORACLE {
			v = t
		} else {
			v = t.Format("2006-01-02 15:04:05")
		}
	case schemas.TimeStampz:
		if dialect.URI().DBType == schemas.MSSQL {
			v = t.Format("2006-01-02T15:04:05.9999999Z07:00")
		} else {
			v = t.Format(time.RFC3339Nano)
		}
	case schemas.BigInt, schemas.Int:
		v = t.Unix()
	default:
		v = t
	}
	return
}

func FormatColumnTime(dialect Dialect, defaultTimeZone *time.Location, col *schemas.Column, t time.Time) (v interface{}) {
	if t.IsZero() {
		if col.Nullable {
			return nil
		}
		return ""
	}

	if col.TimeZone != nil {
		return FormatTime(dialect, col.SQLType.Name, t.In(col.TimeZone))
	}
	return FormatTime(dialect, col.SQLType.Name, t.In(defaultTimeZone))
}
