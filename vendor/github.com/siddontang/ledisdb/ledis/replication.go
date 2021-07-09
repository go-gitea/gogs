package ledis

import (
	"bytes"
	"errors"
	"io"
	"time"

	"github.com/siddontang/go/log"
	"github.com/siddontang/go/snappy"
	"github.com/siddontang/ledisdb/rpl"
	"github.com/siddontang/ledisdb/store"
)

const (
	maxReplLogSize = 1 * 1024 * 1024
)

// For replication error.
var (
	ErrLogMissed = errors.New("log is pured in server")
)

// ReplicationUsed returns whether replication is used or not.
func (l *Ledis) ReplicationUsed() bool {
	return l.r != nil
}

func (l *Ledis) handleReplication() error {
	l.wLock.Lock()
	defer l.wLock.Unlock()

	defer AsyncNotify(l.rDoneCh)

	rl := &rpl.Log{}

	var err error
	for {
		if err = l.r.NextNeedCommitLog(rl); err != nil {
			if err != rpl.ErrNoBehindLog {
				log.Errorf("get next commit log err, %s", err.Error())
				return err
			}

			return nil
		}

		l.rbatch.Rollback()

		if rl.Compression == 1 {
			//todo optimize
			if rl.Data, err = snappy.Decode(nil, rl.Data); err != nil {
				log.Errorf("decode log error %s", err.Error())
				return err
			}
		}

		if bd, err := store.NewBatchData(rl.Data); err != nil {
			log.Errorf("decode batch log error %s", err.Error())
			return err
		} else if err = bd.Replay(l.rbatch); err != nil {
			log.Errorf("replay batch log error %s", err.Error())
		}

		l.commitLock.Lock()
		if err = l.rbatch.Commit(); err != nil {
			log.Errorf("commit log error %s", err.Error())
		} else if err = l.r.UpdateCommitID(rl.ID); err != nil {
			log.Errorf("update commit id error %s", err.Error())
		}

		l.commitLock.Unlock()
		if err != nil {
			return err
		}
	}
}

func (l *Ledis) onReplication() {
	defer l.wg.Done()

	l.noticeReplication()

	for {
		select {
		case <-l.rc:
			l.handleReplication()
		case <-l.quit:
			return
		}
	}
}

// WaitReplication waits replication done
func (l *Ledis) WaitReplication() error {
	if !l.ReplicationUsed() {
		return ErrRplNotSupport

	}

	for i := 0; i < 100; i++ {
		l.noticeReplication()

		select {
		case <-l.rDoneCh:
		case <-l.quit:
			return nil
		}
		time.Sleep(100 * time.Millisecond)

		b, err := l.r.CommitIDBehind()
		if err != nil {
			return err
		} else if !b {
			return nil
		}
	}

	return errors.New("wait replication too many times")
}

// StoreLogsFromReader stores logs from the Reader
func (l *Ledis) StoreLogsFromReader(rb io.Reader) error {
	if !l.ReplicationUsed() {
		return ErrRplNotSupport
	} else if !l.cfg.Readonly {
		return ErrRplInRDWR
	}

	log := &rpl.Log{}

	for {
		if err := log.Decode(rb); err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		if err := l.r.StoreLog(log); err != nil {
			return err
		}

	}

	l.noticeReplication()

	return nil
}

func (l *Ledis) noticeReplication() {
	AsyncNotify(l.rc)
}

// StoreLogsFromData stores logs from data.
func (l *Ledis) StoreLogsFromData(data []byte) error {
	rb := bytes.NewReader(data)

	return l.StoreLogsFromReader(rb)
}

// ReadLogsTo reads logs and write to the Writer.
func (l *Ledis) ReadLogsTo(startLogID uint64, w io.Writer) (n int, nextLogID uint64, err error) {
	if !l.ReplicationUsed() {
		// no replication log
		nextLogID = 0
		err = ErrRplNotSupport
		return
	}

	var firtID, lastID uint64

	firtID, err = l.r.FirstLogID()
	if err != nil {
		return
	}

	if startLogID < firtID {
		err = ErrLogMissed
		return
	}

	lastID, err = l.r.LastLogID()
	if err != nil {
		return
	}

	nextLogID = startLogID

	log := &rpl.Log{}
	for i := startLogID; i <= lastID; i++ {
		if err = l.r.GetLog(i, log); err != nil {
			return
		}

		if err = log.Encode(w); err != nil {
			return
		}

		nextLogID = i + 1

		n += log.Size()

		if n > maxReplLogSize {
			break
		}
	}

	return
}

// ReadLogsToTimeout tries to read events, if no events read,
// tres to wait the new event singal until timeout seconds
func (l *Ledis) ReadLogsToTimeout(startLogID uint64, w io.Writer, timeout int, quitCh chan struct{}) (n int, nextLogID uint64, err error) {
	n, nextLogID, err = l.ReadLogsTo(startLogID, w)
	if err != nil {
		return
	} else if n != 0 {
		return
	}
	//no events read
	select {
	case <-l.r.WaitLog():
	case <-time.After(time.Duration(timeout) * time.Second):
	case <-quitCh:
		return
	}
	return l.ReadLogsTo(startLogID, w)
}

func (l *Ledis) propagate(rl *rpl.Log) {
	for _, h := range l.rhs {
		h(rl)
	}
}

// NewLogEventHandler is the handler to handle new log event.
type NewLogEventHandler func(rl *rpl.Log)

// AddNewLogEventHandler adds the handler for the new log event
func (l *Ledis) AddNewLogEventHandler(h NewLogEventHandler) error {
	if !l.ReplicationUsed() {
		return ErrRplNotSupport
	}

	l.rhs = append(l.rhs, h)

	return nil
}

// ReplicationStat returns the statistics of repliaciton.
func (l *Ledis) ReplicationStat() (*rpl.Stat, error) {
	if !l.ReplicationUsed() {
		return nil, ErrRplNotSupport
	}

	return l.r.Stat()
}
