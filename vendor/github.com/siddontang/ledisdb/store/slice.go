package store

import (
	"github.com/siddontang/ledisdb/store/driver"
)

type Slice interface {
	driver.ISlice
}
