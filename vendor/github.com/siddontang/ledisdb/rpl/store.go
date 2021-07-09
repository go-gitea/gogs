package rpl

import (
	"errors"
)

const (
	InvalidLogID uint64 = 0
)

var (
	ErrLogNotFound    = errors.New("log not found")
	ErrStoreLogID     = errors.New("log id is less")
	ErrNoBehindLog    = errors.New("no behind commit log")
	ErrCommitIDBehind = errors.New("commit id is behind last log id")
)

type LogStore interface {
	GetLog(id uint64, log *Log) error

	FirstID() (uint64, error)
	LastID() (uint64, error)

	// if log id is less than current last id, return error
	StoreLog(log *Log) error

	// Delete logs before n seconds
	PurgeExpired(n int64) error

	Sync() error

	// Clear all logs
	Clear() error

	Close() error
}
