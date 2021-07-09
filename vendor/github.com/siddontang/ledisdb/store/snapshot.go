package store

import (
	"github.com/siddontang/ledisdb/store/driver"
)

type Snapshot struct {
	driver.ISnapshot
	st *Stat
}

func (s *Snapshot) NewIterator() *Iterator {
	it := new(Iterator)
	it.it = s.ISnapshot.NewIterator()
	it.st = s.st

	s.st.IterNum.Add(1)

	return it
}

func (s *Snapshot) Get(key []byte) ([]byte, error) {
	v, err := s.ISnapshot.Get(key)
	s.st.statGet(v, err)
	return v, err
}

func (s *Snapshot) GetSlice(key []byte) (Slice, error) {
	if d, ok := s.ISnapshot.(driver.ISliceGeter); ok {
		v, err := d.GetSlice(key)
		s.st.statGet(v, err)
		return v, err
	} else {
		v, err := s.Get(key)
		if err != nil {
			return nil, err
		} else if v == nil {
			return nil, nil
		} else {
			return driver.GoSlice(v), nil
		}
	}
}

func (s *Snapshot) Close() {
	s.st.SnapshotCloseNum.Add(1)
	s.ISnapshot.Close()
}
