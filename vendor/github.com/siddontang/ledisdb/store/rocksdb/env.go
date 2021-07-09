// +build rocksdb

package rocksdb

// #cgo LDFLAGS: -lrocksdb
// #include "rocksdb/c.h"
import "C"

type Env struct {
	Env *C.rocksdb_env_t
}

func NewDefaultEnv() *Env {
	return &Env{C.rocksdb_create_default_env()}
}

func (env *Env) SetHighPriorityBackgroundThreads(n int) {
	C.rocksdb_env_set_high_priority_background_threads(env.Env, C.int(n))
}

func (env *Env) SetBackgroundThreads(n int) {
	C.rocksdb_env_set_background_threads(env.Env, C.int(n))
}

func (env *Env) Close() {
	C.rocksdb_env_destroy(env.Env)
}
