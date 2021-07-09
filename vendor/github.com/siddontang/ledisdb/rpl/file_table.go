package rpl

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"github.com/siddontang/go/log"
	"github.com/siddontang/go/sync2"
)

var (
	magic             = []byte("\x1c\x1d\xb8\x88\xff\x9e\x45\x55\x40\xf0\x4c\xda\xe0\xce\x47\xde\x65\x48\x71\x17")
	errTableNeedFlush = errors.New("write table need flush")
	errNilHandler     = errors.New("nil write handler")
)

const tableReaderKeepaliveInterval int64 = 30

func fmtTableDataName(base string, index int64) string {
	return path.Join(base, fmt.Sprintf("%08d.data", index))
}

func fmtTableMetaName(base string, index int64) string {
	return path.Join(base, fmt.Sprintf("%08d.meta", index))
}

type tableReader struct {
	sync.Mutex

	base  string
	index int64

	data readFile
	meta readFile

	first uint64
	last  uint64

	lastTime uint32

	lastReadTime sync2.AtomicInt64

	useMmap bool
}

func newTableReader(base string, index int64, useMmap bool) (*tableReader, error) {
	if index <= 0 {
		return nil, fmt.Errorf("invalid index %d", index)
	}
	t := new(tableReader)
	t.base = base
	t.index = index

	t.useMmap = useMmap

	var err error

	if err = t.check(); err != nil {
		log.Errorf("check %d error: %s, try to repair", t.index, err.Error())

		if err = t.repair(); err != nil {
			log.Errorf("repair %d error: %s", t.index, err.Error())
			return nil, err
		}
	}

	t.close()

	return t, nil
}

func (t *tableReader) String() string {
	return fmt.Sprintf("%d", t.index)
}

func (t *tableReader) Close() {
	t.Lock()

	t.close()

	t.Unlock()
}

func (t *tableReader) close() {
	if t.data != nil {
		t.data.Close()
		t.data = nil
	}

	if t.meta != nil {
		t.meta.Close()
		t.meta = nil
	}
}

func (t *tableReader) Keepalived() bool {
	l := t.lastReadTime.Get()
	if l > 0 && time.Now().Unix()-l > tableReaderKeepaliveInterval {
		return false
	}

	return true
}

func (t *tableReader) getLogPos(index int) (uint32, error) {
	var buf [4]byte
	if _, err := t.meta.ReadAt(buf[0:4], int64(index)*4); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buf[0:4]), nil
}

func (t *tableReader) checkData() error {
	var err error
	//check will use raw file mode
	if t.data, err = newReadFile(false, fmtTableDataName(t.base, t.index)); err != nil {
		return err
	}

	if t.data.Size() < len(magic) {
		return fmt.Errorf("data file %s size %d too short", t.data.Name(), t.data.Size())
	}

	buf := make([]byte, len(magic))
	if _, err := t.data.ReadAt(buf, int64(t.data.Size()-len(magic))); err != nil {
		return err
	}

	if !bytes.Equal(magic, buf) {
		return fmt.Errorf("data file %s invalid magic data %q", t.data.Name(), buf)
	}

	return nil
}

func (t *tableReader) checkMeta() error {
	var err error
	//check will use raw file mode
	if t.meta, err = newReadFile(false, fmtTableMetaName(t.base, t.index)); err != nil {
		return err
	}

	if t.meta.Size()%4 != 0 || t.meta.Size() == 0 {
		return fmt.Errorf("meta file %s invalid offset len %d, must 4 multiple and not 0", t.meta.Name(), t.meta.Size())
	}

	return nil
}

func (t *tableReader) check() error {
	var err error

	if err := t.checkMeta(); err != nil {
		return err
	}

	if err := t.checkData(); err != nil {
		return err
	}

	firstLogPos, _ := t.getLogPos(0)
	lastLogPos, _ := t.getLogPos(t.meta.Size()/4 - 1)

	if firstLogPos != 0 {
		return fmt.Errorf("invalid first log pos %d, must 0", firstLogPos)
	}

	var l Log
	if _, err = t.decodeLogHead(&l, t.data, int64(firstLogPos)); err != nil {
		return fmt.Errorf("decode first log err %s", err.Error())
	}

	t.first = l.ID
	var n int64
	if n, err = t.decodeLogHead(&l, t.data, int64(lastLogPos)); err != nil {
		return fmt.Errorf("decode last log err %s", err.Error())
	} else if n+int64(len(magic)) != int64(t.data.Size()) {
		return fmt.Errorf("extra log data at offset %d", n)
	}

	t.last = l.ID
	t.lastTime = l.CreateTime

	if t.first > t.last {
		return fmt.Errorf("invalid log table first %d > last %d", t.first, t.last)
	} else if (t.last - t.first + 1) != uint64(t.meta.Size()/4) {
		return fmt.Errorf("invalid log table, first %d, last %d, and log num %d", t.first, t.last, t.meta.Size()/4)
	}

	return nil
}

func (t *tableReader) repair() error {
	t.close()

	var err error
	var data writeFile
	var meta writeFile

	//repair will use raw file mode
	data, err = newWriteFile(false, fmtTableDataName(t.base, t.index), 0)
	data.SetOffset(int64(data.Size()))

	meta, err = newWriteFile(false, fmtTableMetaName(t.base, t.index), int64(defaultLogNumInFile*4))

	var l Log
	var pos int64 = 0
	var nextPos int64 = 0
	b := make([]byte, 4)

	t.first = 0
	t.last = 0

	for {
		nextPos, err = t.decodeLogHead(&l, data, pos)
		if err != nil {
			//if error, we may lost all logs from pos
			log.Errorf("%s may lost logs from %d", data.Name(), pos)
			break
		}

		if l.ID == 0 {
			log.Errorf("%s may lost logs from %d, invalid log 0", data.Name(), pos)
			break
		}

		if t.first == 0 {
			t.first = l.ID
		}

		if t.last == 0 {
			t.last = l.ID
		} else if l.ID <= t.last {
			log.Errorf("%s may lost logs from %d, invalid logid %d", t.data.Name(), pos, l.ID)
			break
		}

		t.last = l.ID
		t.lastTime = l.CreateTime

		binary.BigEndian.PutUint32(b, uint32(pos))
		meta.Write(b)

		pos = nextPos

		t.lastTime = l.CreateTime
	}

	var e error
	if err := meta.Close(); err != nil {
		e = err
	}

	data.SetOffset(pos)

	if _, err = data.Write(magic); err != nil {
		log.Errorf("write magic error %s", err.Error())
	}

	if err = data.Close(); err != nil {
		return err
	}

	return e
}

func (t *tableReader) decodeLogHead(l *Log, r io.ReaderAt, pos int64) (int64, error) {
	dataLen, err := l.DecodeHeadAt(r, pos)
	if err != nil {
		return 0, err
	}

	return pos + int64(l.HeadSize()) + int64(dataLen), nil
}

func (t *tableReader) GetLog(id uint64, l *Log) error {
	if id < t.first || id > t.last {
		return ErrLogNotFound
	}

	t.lastReadTime.Set(time.Now().Unix())

	t.Lock()

	if err := t.openTable(); err != nil {
		t.close()
		t.Unlock()
		return err
	}
	t.Unlock()

	pos, err := t.getLogPos(int(id - t.first))
	if err != nil {
		return err
	}

	if err := l.DecodeAt(t.data, int64(pos)); err != nil {
		return err
	} else if l.ID != id {
		return fmt.Errorf("invalid log id %d != %d", l.ID, id)
	}

	return nil
}

func (t *tableReader) openTable() error {
	var err error
	if t.data == nil {
		if t.data, err = newReadFile(t.useMmap, fmtTableDataName(t.base, t.index)); err != nil {
			return err
		}
	}

	if t.meta == nil {
		if t.meta, err = newReadFile(t.useMmap, fmtTableMetaName(t.base, t.index)); err != nil {
			return err
		}

	}

	return nil
}

type tableWriter struct {
	sync.RWMutex

	data writeFile
	meta writeFile

	base  string
	index int64

	first    uint64
	last     uint64
	lastTime uint32

	maxLogSize int64

	closed bool

	syncType int

	posBuf []byte

	useMmap bool
}

func newTableWriter(base string, index int64, maxLogSize int64, useMmap bool) *tableWriter {
	if index <= 0 {
		panic(fmt.Errorf("invalid index %d", index))
	}

	t := new(tableWriter)

	t.base = base
	t.index = index

	t.maxLogSize = maxLogSize

	t.closed = false

	t.posBuf = make([]byte, 4)

	t.useMmap = useMmap

	return t
}

func (t *tableWriter) String() string {
	return fmt.Sprintf("%d", t.index)
}

func (t *tableWriter) SetMaxLogSize(s int64) {
	t.maxLogSize = s
}

func (t *tableWriter) SetSyncType(tp int) {
	t.syncType = tp
}

func (t *tableWriter) close() {
	if t.meta != nil {
		if err := t.meta.Close(); err != nil {
			log.Fatalf("close log meta error %s", err.Error())
		}
		t.meta = nil
	}

	if t.data != nil {
		if _, err := t.data.Write(magic); err != nil {
			log.Fatalf("write magic error %s", err.Error())
		}

		if err := t.data.Close(); err != nil {
			log.Fatalf("close log data error %s", err.Error())
		}
		t.data = nil
	}
}

func (t *tableWriter) Close() {
	t.Lock()
	t.closed = true

	t.close()
	t.Unlock()
}

func (t *tableWriter) First() uint64 {
	t.Lock()
	id := t.first
	t.Unlock()
	return id
}

func (t *tableWriter) Last() uint64 {
	t.Lock()
	id := t.last
	t.Unlock()
	return id
}

func (t *tableWriter) Flush() (*tableReader, error) {
	t.Lock()

	if t.data == nil || t.meta == nil {
		t.Unlock()
		return nil, errNilHandler
	}

	tr := new(tableReader)
	tr.base = t.base
	tr.index = t.index

	tr.first = t.first
	tr.last = t.last
	tr.lastTime = t.lastTime
	tr.useMmap = t.useMmap

	t.close()

	t.first = 0
	t.last = 0
	t.index = t.index + 1

	t.Unlock()

	return tr, nil
}

func (t *tableWriter) StoreLog(l *Log) error {
	t.Lock()
	err := t.storeLog(l)
	t.Unlock()

	return err
}

func (t *tableWriter) openFile() error {
	var err error
	if t.data == nil {
		if t.data, err = newWriteFile(t.useMmap, fmtTableDataName(t.base, t.index), t.maxLogSize+t.maxLogSize/10+int64(len(magic))); err != nil {
			return err
		}
	}

	if t.meta == nil {
		if t.meta, err = newWriteFile(t.useMmap, fmtTableMetaName(t.base, t.index), int64(defaultLogNumInFile*4)); err != nil {
			return err
		}
	}
	return err
}

func (t *tableWriter) storeLog(l *Log) error {
	if l.ID == 0 {
		return ErrStoreLogID
	}

	if t.closed {
		return fmt.Errorf("table writer is closed")
	}

	if t.last > 0 && l.ID != t.last+1 {
		return ErrStoreLogID
	}

	if t.data != nil && t.data.Offset() > t.maxLogSize {
		return errTableNeedFlush
	}

	var err error
	if err = t.openFile(); err != nil {
		return err
	}

	offsetPos := t.data.Offset()
	if err = l.Encode(t.data); err != nil {
		return err
	}

	binary.BigEndian.PutUint32(t.posBuf, uint32(offsetPos))
	if _, err = t.meta.Write(t.posBuf); err != nil {
		return err
	}

	if t.first == 0 {
		t.first = l.ID
	}

	t.last = l.ID
	t.lastTime = l.CreateTime

	if t.syncType == 2 {
		if err := t.data.Sync(); err != nil {
			log.Errorf("sync table error %s", err.Error())
		}
	}

	return nil
}

func (t *tableWriter) GetLog(id uint64, l *Log) error {
	t.RLock()
	defer t.RUnlock()

	if id < t.first || id > t.last {
		return ErrLogNotFound
	}

	var buf [4]byte
	if _, err := t.meta.ReadAt(buf[0:4], int64((id-t.first)*4)); err != nil {
		return err
	}

	offset := binary.BigEndian.Uint32(buf[0:4])

	if err := l.DecodeAt(t.data, int64(offset)); err != nil {
		return err
	} else if l.ID != id {
		return fmt.Errorf("invalid log id %d != %d", id, l.ID)
	}

	return nil
}

func (t *tableWriter) Sync() error {
	t.Lock()

	var err error
	if t.data != nil {
		err = t.data.Sync()
		t.Unlock()
		return err
	}

	if t.meta != nil {
		err = t.meta.Sync()
	}

	t.Unlock()

	return err
}
