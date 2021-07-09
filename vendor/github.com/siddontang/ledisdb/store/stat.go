package store

import (
	"github.com/siddontang/go/sync2"
)

type Stat struct {
	GetNum               sync2.AtomicInt64
	GetMissingNum        sync2.AtomicInt64
	GetTotalTime         sync2.AtomicDuration
	PutNum               sync2.AtomicInt64
	DeleteNum            sync2.AtomicInt64
	IterNum              sync2.AtomicInt64
	IterSeekNum          sync2.AtomicInt64
	IterCloseNum         sync2.AtomicInt64
	SnapshotNum          sync2.AtomicInt64
	SnapshotCloseNum     sync2.AtomicInt64
	BatchNum             sync2.AtomicInt64
	BatchCommitNum       sync2.AtomicInt64
	BatchCommitTotalTime sync2.AtomicDuration
	TxNum                sync2.AtomicInt64
	TxCommitNum          sync2.AtomicInt64
	TxCloseNum           sync2.AtomicInt64
	CompactNum           sync2.AtomicInt64
	CompactTotalTime     sync2.AtomicDuration
}

func (st *Stat) statGet(v interface{}, err error) {
	st.GetNum.Add(1)
	if v == nil && err == nil {
		st.GetMissingNum.Add(1)
	}
}

func (st *Stat) Reset() {
	*st = Stat{}
}
