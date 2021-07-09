package ledis

import (
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/siddontang/go/filelock"
	"github.com/siddontang/go/log"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/rpl"
	"github.com/siddontang/ledisdb/store"
)

// Ledis is the core structure to handle the database.
type Ledis struct {
	cfg *config.Config

	ldb *store.DB

	dbLock sync.Mutex
	dbs    map[int]*DB

	quit chan struct{}
	wg   sync.WaitGroup

	//for replication
	r       *rpl.Replication
	rc      chan struct{}
	rbatch  *store.WriteBatch
	rDoneCh chan struct{}
	rhs     []NewLogEventHandler

	wLock      sync.RWMutex //allow one write at same time
	commitLock sync.Mutex   //allow one write commit at same time

	lock io.Closer

	ttlCheckers  []*ttlChecker
	ttlCheckerCh chan *ttlChecker
}

// Open opens the Ledis with a config.
func Open(cfg *config.Config) (*Ledis, error) {
	if len(cfg.DataDir) == 0 {
		cfg.DataDir = config.DefaultDataDir
	}

	if cfg.Databases == 0 {
		cfg.Databases = 16
	} else if cfg.Databases > MaxDatabases {
		cfg.Databases = MaxDatabases
	}

	os.MkdirAll(cfg.DataDir, 0755)

	var err error

	l := new(Ledis)
	l.cfg = cfg

	if l.lock, err = filelock.Lock(path.Join(cfg.DataDir, "LOCK")); err != nil {
		return nil, err
	}

	l.quit = make(chan struct{})

	if l.ldb, err = store.Open(cfg); err != nil {
		return nil, err
	}

	if cfg.UseReplication {
		if l.r, err = rpl.NewReplication(cfg); err != nil {
			return nil, err
		}

		l.rc = make(chan struct{}, 1)
		l.rbatch = l.ldb.NewWriteBatch()
		l.rDoneCh = make(chan struct{}, 1)

		l.wg.Add(1)
		go l.onReplication()

		//first we must try wait all replication ok
		//maybe some logs are not committed
		l.WaitReplication()
	} else {
		l.r = nil
	}

	l.dbs = make(map[int]*DB, 16)

	l.checkTTL()

	return l, nil
}

// Close closes the Ledis.
func (l *Ledis) Close() {
	close(l.quit)
	l.wg.Wait()

	l.ldb.Close()

	if l.r != nil {
		l.r.Close()
		//l.r = nil
	}

	if l.lock != nil {
		l.lock.Close()
		//l.lock = nil
	}
}

// Select chooses a database.
func (l *Ledis) Select(index int) (*DB, error) {
	if index < 0 || index >= l.cfg.Databases {
		return nil, fmt.Errorf("invalid db index %d, must in [0, %d]", index, l.cfg.Databases-1)
	}

	l.dbLock.Lock()
	defer l.dbLock.Unlock()

	db, ok := l.dbs[index]
	if ok {
		return db, nil
	}

	db = l.newDB(index)
	l.dbs[index] = db

	go func(db *DB) {
		l.ttlCheckerCh <- db.ttlChecker
	}(db)

	return db, nil
}

// FlushAll will clear all data and replication logs
func (l *Ledis) FlushAll() error {
	l.wLock.Lock()
	defer l.wLock.Unlock()

	return l.flushAll()
}

func (l *Ledis) flushAll() error {
	it := l.ldb.NewIterator()
	defer it.Close()

	it.SeekToFirst()

	w := l.ldb.NewWriteBatch()
	defer w.Rollback()

	n := 0
	for ; it.Valid(); it.Next() {
		n++
		if n == 10000 {
			if err := w.Commit(); err != nil {
				log.Fatalf("flush all commit error: %s", err.Error())
				return err
			}
			n = 0
		}
		w.Delete(it.RawKey())
	}

	if err := w.Commit(); err != nil {
		log.Fatalf("flush all commit error: %s", err.Error())
		return err
	}

	if l.r != nil {
		if err := l.r.Clear(); err != nil {
			log.Fatalf("flush all replication clear error: %s", err.Error())
			return err
		}
	}

	return nil
}

// IsReadOnly returns whether Ledis is read only or not.
func (l *Ledis) IsReadOnly() bool {
	if l.cfg.GetReadonly() {
		return true
	} else if l.r != nil {
		if b, _ := l.r.CommitIDBehind(); b {
			return true
		}
	}
	return false
}

func (l *Ledis) checkTTL() {
	l.ttlCheckers = make([]*ttlChecker, 0, 16)
	l.ttlCheckerCh = make(chan *ttlChecker, 16)

	if l.cfg.TTLCheckInterval == 0 {
		l.cfg.TTLCheckInterval = 1
	}

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		tick := time.NewTicker(time.Duration(l.cfg.TTLCheckInterval) * time.Second)
		defer tick.Stop()

		for {
			select {
			case <-tick.C:
				if l.IsReadOnly() {
					break
				}

				for _, c := range l.ttlCheckers {
					c.check()
				}
			case c := <-l.ttlCheckerCh:
				l.ttlCheckers = append(l.ttlCheckers, c)
				c.check()
			case <-l.quit:
				return
			}
		}

	}()

}

// StoreStat returns the statistics.
func (l *Ledis) StoreStat() *store.Stat {
	return l.ldb.Stat()
}

// CompactStore compacts the backend storage.
func (l *Ledis) CompactStore() error {
	l.wLock.Lock()
	defer l.wLock.Unlock()

	return l.ldb.Compact()
}
