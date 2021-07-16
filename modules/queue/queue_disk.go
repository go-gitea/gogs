// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"

	"code.gitea.io/gitea/modules/nosql"

	"gitea.com/lunny/levelqueue"
)

// LevelQueueType is the type for level queue
const LevelQueueType Type = "level"

// LevelQueueConfiguration is the configuration for a LevelQueue
type LevelQueueConfiguration struct {
	ByteFIFOQueueConfiguration
	DataDir          string
	ConnectionString string
	QueueName        string
}

// LevelQueue implements a disk library queue
type LevelQueue struct {
	*ByteFIFOQueue
}

// NewLevelQueue creates a ledis local queue
func NewLevelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(LevelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LevelQueueConfiguration)

	if len(config.ConnectionString) == 0 {
		config.ConnectionString = config.DataDir
	}
	config.WaitOnEmpty = true

	byteFIFO, err := NewLevelQueueByteFIFO(config.ConnectionString, config.QueueName)
	if err != nil {
		return nil, err
	}

	byteFIFOQueue, err := NewByteFIFOQueue(LevelQueueType, byteFIFO, handle, config.ByteFIFOQueueConfiguration, exemplar)
	if err != nil {
		return nil, err
	}

	queue := &LevelQueue{
		ByteFIFOQueue: byteFIFOQueue,
	}
	queue.qid = GetManager().Add(queue, LevelQueueType, config, exemplar)
	return queue, nil
}

var _ ByteFIFO = &LevelQueueByteFIFO{}

// LevelQueueByteFIFO represents a ByteFIFO formed from a LevelQueue
type LevelQueueByteFIFO struct {
	internal   *levelqueue.Queue
	connection string
}

// NewLevelQueueByteFIFO creates a ByteFIFO formed from a LevelQueue
func NewLevelQueueByteFIFO(connection, prefix string) (*LevelQueueByteFIFO, error) {
	db, err := nosql.GetManager().GetLevelDB(connection)
	if err != nil {
		return nil, err
	}

	internal, err := levelqueue.NewQueue(db, []byte(prefix), false)
	if err != nil {
		return nil, err
	}

	return &LevelQueueByteFIFO{
		connection: connection,
		internal:   internal,
	}, nil
}

// PushFunc will push data into the fifo
func (fifo *LevelQueueByteFIFO) PushFunc(ctx context.Context, data []byte, fn func() error) error {
	if fn != nil {
		if err := fn(); err != nil {
			return err
		}
	}
	return fifo.internal.LPush(data)
}

// PushBack pushes data to the top of the fifo
func (fifo *LevelQueueByteFIFO) PushBack(ctx context.Context, data []byte) error {
	return fifo.internal.RPush(data)
}

// Pop pops data from the start of the fifo
func (fifo *LevelQueueByteFIFO) Pop(ctx context.Context) ([]byte, error) {
	data, err := fifo.internal.RPop()
	if err != nil && err != levelqueue.ErrNotFound {
		return nil, err
	}
	return data, nil
}

// Close this fifo
func (fifo *LevelQueueByteFIFO) Close() error {
	err := fifo.internal.Close()
	_ = nosql.GetManager().CloseLevelDB(fifo.connection)
	return err
}

// Len returns the length of the fifo
func (fifo *LevelQueueByteFIFO) Len(ctx context.Context) int64 {
	return fifo.internal.Len()
}

func init() {
	queuesMap[LevelQueueType] = NewLevelQueue
}
