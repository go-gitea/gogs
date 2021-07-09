package store

import (
	"time"

	"github.com/siddontang/ledisdb/store/driver"
	"github.com/syndtr/goleveldb/leveldb"
)

type WriteBatch struct {
	wb driver.IWriteBatch
	st *Stat

	putNum    int64
	deleteNum int64
	db        *DB

	data *BatchData
}

func (wb *WriteBatch) Close() {
	wb.wb.Close()
}

func (wb *WriteBatch) Put(key []byte, value []byte) {
	wb.putNum++
	wb.wb.Put(key, value)
}

func (wb *WriteBatch) Delete(key []byte) {
	wb.deleteNum++
	wb.wb.Delete(key)
}

func (wb *WriteBatch) Commit() error {
	wb.st.BatchCommitNum.Add(1)
	wb.st.PutNum.Add(wb.putNum)
	wb.st.DeleteNum.Add(wb.deleteNum)
	wb.putNum = 0
	wb.deleteNum = 0

	var err error
	t := time.Now()
	if wb.db == nil || !wb.db.needSyncCommit() {
		err = wb.wb.Commit()
	} else {
		err = wb.wb.SyncCommit()
	}

	wb.st.BatchCommitTotalTime.Add(time.Now().Sub(t))

	return err
}

func (wb *WriteBatch) Rollback() error {
	wb.putNum = 0
	wb.deleteNum = 0

	return wb.wb.Rollback()
}

// the data will be undefined after commit or rollback
func (wb *WriteBatch) BatchData() *BatchData {
	data := wb.wb.Data()
	if wb.data == nil {
		wb.data = new(BatchData)
	}

	wb.data.Load(data)
	return wb.data
}

func (wb *WriteBatch) Data() []byte {
	b := wb.BatchData()
	return b.Data()
}

/*
	see leveldb batch data format for more information
*/

type BatchData struct {
	leveldb.Batch
}

func NewBatchData(data []byte) (*BatchData, error) {
	b := new(BatchData)

	if err := b.Load(data); err != nil {
		return nil, err
	}

	return b, nil
}

func (d *BatchData) Data() []byte {
	return d.Dump()
}

func (d *BatchData) Reset() {
	d.Batch.Reset()
}

type BatchDataReplay interface {
	Put(key, value []byte)
	Delete(key []byte)
}

type BatchItem struct {
	Key   []byte
	Value []byte
}

type batchItems []BatchItem

func (bs *batchItems) Put(key, value []byte) {
	*bs = append(*bs, BatchItem{key, value})
}

func (bs *batchItems) Delete(key []byte) {
	*bs = append(*bs, BatchItem{key, nil})
}

func (d *BatchData) Replay(r BatchDataReplay) error {
	return d.Batch.Replay(r)
}

func (d *BatchData) Items() ([]BatchItem, error) {
	is := make(batchItems, 0, d.Len())

	if err := d.Replay(&is); err != nil {
		return nil, err
	}

	return []BatchItem(is), nil
}
