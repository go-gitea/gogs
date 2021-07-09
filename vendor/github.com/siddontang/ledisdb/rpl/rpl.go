package rpl

import (
	"encoding/binary"
	"os"
	"path"
	"sync"
	"time"

	"github.com/siddontang/go/log"
	"github.com/siddontang/go/snappy"
	"github.com/siddontang/ledisdb/config"
)

type Stat struct {
	FirstID  uint64
	LastID   uint64
	CommitID uint64
}

type Replication struct {
	m sync.Mutex

	cfg *config.Config

	s LogStore

	commitID  uint64
	commitLog *os.File

	quit chan struct{}

	wg sync.WaitGroup

	nc chan struct{}

	ncm sync.Mutex
}

func NewReplication(cfg *config.Config) (*Replication, error) {
	if len(cfg.Replication.Path) == 0 {
		cfg.Replication.Path = path.Join(cfg.DataDir, "rpl")
	}

	base := cfg.Replication.Path

	r := new(Replication)

	r.quit = make(chan struct{})
	r.nc = make(chan struct{})

	r.cfg = cfg

	var err error

	switch cfg.Replication.StoreName {
	case "goleveldb":
		if r.s, err = NewGoLevelDBStore(path.Join(base, "wal"), cfg.Replication.SyncLog); err != nil {
			return nil, err
		}
	default:
		if r.s, err = NewFileStore(path.Join(base, "ldb"), cfg); err != nil {
			return nil, err
		}
	}

	if r.commitLog, err = os.OpenFile(path.Join(base, "commit.log"), os.O_RDWR|os.O_CREATE, 0644); err != nil {
		return nil, err
	}

	if s, _ := r.commitLog.Stat(); s.Size() == 0 {
		r.commitID = 0
	} else if err = binary.Read(r.commitLog, binary.BigEndian, &r.commitID); err != nil {
		return nil, err
	}

	log.Infof("staring replication with commit ID %d", r.commitID)

	r.wg.Add(1)
	go r.run()

	return r, nil
}

func (r *Replication) Close() error {
	close(r.quit)

	r.wg.Wait()

	r.m.Lock()
	defer r.m.Unlock()

	log.Infof("closing replication with commit ID %d", r.commitID)

	if r.s != nil {
		r.s.Close()
		r.s = nil
	}

	if err := r.updateCommitID(r.commitID, true); err != nil {
		log.Errorf("update commit id err %s", err.Error())
	}

	if r.commitLog != nil {
		r.commitLog.Close()
		r.commitLog = nil
	}

	return nil
}

func (r *Replication) Log(data []byte) (*Log, error) {
	if r.cfg.Replication.Compression {
		//todo optimize
		var err error
		if data, err = snappy.Encode(nil, data); err != nil {
			return nil, err
		}
	}

	r.m.Lock()

	lastID, err := r.s.LastID()
	if err != nil {
		r.m.Unlock()
		return nil, err
	}

	commitId := r.commitID
	if lastID < commitId {
		lastID = commitId
	} else if lastID > commitId {
		r.m.Unlock()
		return nil, ErrCommitIDBehind
	}

	l := new(Log)
	l.ID = lastID + 1
	l.CreateTime = uint32(time.Now().Unix())

	if r.cfg.Replication.Compression {
		l.Compression = 1
	} else {
		l.Compression = 0
	}

	l.Data = data

	if err = r.s.StoreLog(l); err != nil {
		r.m.Unlock()
		return nil, err
	}

	r.m.Unlock()

	r.ncm.Lock()
	close(r.nc)
	r.nc = make(chan struct{})
	r.ncm.Unlock()

	return l, nil
}

func (r *Replication) WaitLog() <-chan struct{} {
	r.ncm.Lock()
	ch := r.nc
	r.ncm.Unlock()
	return ch
}

func (r *Replication) StoreLog(log *Log) error {
	r.m.Lock()
	err := r.s.StoreLog(log)
	r.m.Unlock()

	return err
}

func (r *Replication) FirstLogID() (uint64, error) {
	r.m.Lock()
	id, err := r.s.FirstID()
	r.m.Unlock()

	return id, err
}

func (r *Replication) LastLogID() (uint64, error) {
	r.m.Lock()
	id, err := r.s.LastID()
	r.m.Unlock()
	return id, err
}

func (r *Replication) LastCommitID() (uint64, error) {
	r.m.Lock()
	id := r.commitID
	r.m.Unlock()
	return id, nil
}

func (r *Replication) UpdateCommitID(id uint64) error {
	r.m.Lock()
	err := r.updateCommitID(id, r.cfg.Replication.SyncLog == 2)
	r.m.Unlock()

	return err
}

func (r *Replication) Stat() (*Stat, error) {
	r.m.Lock()
	defer r.m.Unlock()

	s := &Stat{}
	var err error

	if s.FirstID, err = r.s.FirstID(); err != nil {
		return nil, err
	}

	if s.LastID, err = r.s.LastID(); err != nil {
		return nil, err
	}

	s.CommitID = r.commitID
	return s, nil
}

func (r *Replication) updateCommitID(id uint64, force bool) error {
	if force {
		if _, err := r.commitLog.Seek(0, os.SEEK_SET); err != nil {
			return err
		}

		if err := binary.Write(r.commitLog, binary.BigEndian, id); err != nil {
			return err
		}
	}

	r.commitID = id

	return nil
}

func (r *Replication) CommitIDBehind() (bool, error) {
	r.m.Lock()

	id, err := r.s.LastID()
	if err != nil {
		r.m.Unlock()
		return false, err
	}

	behind := id > r.commitID
	r.m.Unlock()

	return behind, nil
}

func (r *Replication) GetLog(id uint64, log *Log) error {
	return r.s.GetLog(id, log)
}

func (r *Replication) NextNeedCommitLog(log *Log) error {
	r.m.Lock()
	defer r.m.Unlock()

	id, err := r.s.LastID()
	if err != nil {
		return err
	}

	if id <= r.commitID {
		return ErrNoBehindLog
	}

	return r.s.GetLog(r.commitID+1, log)

}

func (r *Replication) Clear() error {
	return r.ClearWithCommitID(0)
}

func (r *Replication) ClearWithCommitID(id uint64) error {
	r.m.Lock()
	defer r.m.Unlock()

	if err := r.s.Clear(); err != nil {
		return err
	}

	return r.updateCommitID(id, true)
}

func (r *Replication) run() {
	defer r.wg.Done()

	syncTc := time.NewTicker(1 * time.Second)
	purgeTc := time.NewTicker(1 * time.Hour)

	for {
		select {
		case <-purgeTc.C:
			n := (r.cfg.Replication.ExpiredLogDays * 24 * 3600)
			r.m.Lock()
			err := r.s.PurgeExpired(int64(n))
			r.m.Unlock()
			if err != nil {
				log.Errorf("purge expired log error %s", err.Error())
			}
		case <-syncTc.C:
			if r.cfg.Replication.SyncLog == 1 {
				r.m.Lock()
				err := r.s.Sync()
				r.m.Unlock()
				if err != nil {
					log.Errorf("sync store error %s", err.Error())
				}
			}
			if r.cfg.Replication.SyncLog != 2 {
				//we will sync commit id every 1 second
				r.m.Lock()
				err := r.updateCommitID(r.commitID, true)
				r.m.Unlock()

				if err != nil {
					log.Errorf("sync commitid error %s", err.Error())
				}
			}
		case <-r.quit:
			syncTc.Stop()
			purgeTc.Stop()
			return
		}
	}
}
