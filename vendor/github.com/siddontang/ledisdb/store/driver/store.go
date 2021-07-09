package driver

import (
	"fmt"

	"github.com/siddontang/ledisdb/config"
)

type Store interface {
	String() string
	Open(path string, cfg *config.Config) (IDB, error)
	Repair(path string, cfg *config.Config) error
}

var dbs = map[string]Store{}

func Register(s Store) {
	name := s.String()
	if _, ok := dbs[name]; ok {
		panic(fmt.Errorf("store %s is registered", s))
	}

	dbs[name] = s
}

func ListStores() []string {
	s := []string{}
	for k := range dbs {
		s = append(s, k)
	}

	return s
}

func GetStore(cfg *config.Config) (Store, error) {
	if len(cfg.DBName) == 0 {
		cfg.DBName = config.DefaultDBName
	}

	s, ok := dbs[cfg.DBName]
	if !ok {
		return nil, fmt.Errorf("store %s is not registered", cfg.DBName)
	}

	return s, nil
}
