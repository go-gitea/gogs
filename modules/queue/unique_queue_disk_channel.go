// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// PersistableChannelUniqueQueueType is the type for persistable queue
const PersistableChannelUniqueQueueType Type = "unique-persistable-channel"

// PersistableChannelUniqueQueueConfiguration is the configuration for a PersistableChannelUniqueQueue
type PersistableChannelUniqueQueueConfiguration struct {
	Name         string
	DataDir      string
	BatchLength  int
	QueueLength  int
	Timeout      time.Duration
	MaxAttempts  int
	Workers      int
	MaxWorkers   int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
}

// PersistableChannelUniqueQueue wraps a channel queue and level queue together
//
// Please note that this Queue does not guarantee that a particular
// task cannot be processed twice or more at the same time. Uniqueness is
// only guaranteed whilst the task is waiting in the queue.
type PersistableChannelUniqueQueue struct {
	channelQueue *ChannelUniqueQueue
	delayedStarter
	lock   sync.Mutex
	closed chan struct{}
}

// NewPersistableChannelUniqueQueue creates a wrapped batched channel queue with persistable level queue backend when shutting down
// This differs from a wrapped queue in that the persistent queue is only used to persist at shutdown/terminate
func NewPersistableChannelUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(PersistableChannelUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(PersistableChannelUniqueQueueConfiguration)

	queue := &PersistableChannelUniqueQueue{
		closed: make(chan struct{}),
	}

	wrappedHandle := func(data ...Data) (failed []Data) {
		for _, unhandled := range handle(data...) {
			if fail := queue.PushBack(unhandled); fail != nil {
				failed = append(failed, fail)
			}
		}
		return
	}

	channelUniqueQueue, err := NewChannelUniqueQueue(wrappedHandle, ChannelUniqueQueueConfiguration{
		WorkerPoolConfiguration: WorkerPoolConfiguration{
			QueueLength:  config.QueueLength,
			BatchLength:  config.BatchLength,
			BlockTimeout: config.BlockTimeout,
			BoostTimeout: config.BoostTimeout,
			BoostWorkers: config.BoostWorkers,
			MaxWorkers:   config.MaxWorkers,
		},
		Workers: config.Workers,
		Name:    config.Name + "-channel",
	}, exemplar)
	if err != nil {
		return nil, err
	}

	// the level backend only needs temporary workers to catch up with the previously dropped work
	levelCfg := LevelUniqueQueueConfiguration{
		ByteFIFOQueueConfiguration: ByteFIFOQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  config.QueueLength,
				BatchLength:  config.BatchLength,
				BlockTimeout: 1 * time.Second,
				BoostTimeout: 5 * time.Minute,
				BoostWorkers: 1,
				MaxWorkers:   5,
			},
			Workers: 0,
			Name:    config.Name + "-level",
		},
		DataDir: config.DataDir,
	}

	queue.channelQueue = channelUniqueQueue.(*ChannelUniqueQueue)

	levelQueue, err := NewLevelUniqueQueue(func(data ...Data) []Data {
		for _, datum := range data {
			err := queue.Push(datum)
			if err != nil && err != ErrAlreadyInQueue {
				log.Error("Unable push to channelled queue: %v", err)
			}
		}
		return nil
	}, levelCfg, exemplar)
	if err == nil {
		queue.delayedStarter = delayedStarter{
			internal: levelQueue.(*LevelUniqueQueue),
			name:     config.Name,
		}

		_ = GetManager().Add(queue, PersistableChannelUniqueQueueType, config, exemplar)
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	queue.delayedStarter = delayedStarter{
		cfg:         levelCfg,
		underlying:  LevelUniqueQueueType,
		timeout:     config.Timeout,
		maxAttempts: config.MaxAttempts,
		name:        config.Name,
	}
	_ = GetManager().Add(queue, PersistableChannelUniqueQueueType, config, exemplar)
	return queue, nil
}

// Name returns the name of this queue
func (q *PersistableChannelUniqueQueue) Name() string {
	return q.delayedStarter.name
}

// Push will push the indexer data to queue
func (q *PersistableChannelUniqueQueue) Push(data Data) error {
	return q.PushFunc(data, nil)
}

// PushFunc will push the indexer data to queue
func (q *PersistableChannelUniqueQueue) PushFunc(data Data, fn func() error) error {
	select {
	case <-q.closed:
		return q.internal.(UniqueQueue).PushFunc(data, fn)
	default:
		return q.channelQueue.PushFunc(data, fn)
	}
}

// PushBack will push the indexer data to queue
func (q *PersistableChannelUniqueQueue) PushBack(data Data) error {
	select {
	case <-q.closed:
		if pbr, ok := q.internal.(PushBackable); ok {
			return pbr.PushBack(data)
		}
		return q.internal.Push(data)
	default:
		return q.channelQueue.Push(data)
	}
}

// Has will test if the queue has the data
func (q *PersistableChannelUniqueQueue) Has(data Data) (bool, error) {
	// This is more difficult...
	has, err := q.channelQueue.Has(data)
	if err != nil || has {
		return has, err
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal == nil {
		return false, nil
	}
	return q.internal.(UniqueQueue).Has(data)
}

// Run starts to run the queue
func (q *PersistableChannelUniqueQueue) Run(atShutdown, atTerminate func(func())) {
	log.Debug("PersistableChannelUniqueQueue: %s Starting", q.delayedStarter.name)

	q.lock.Lock()
	if q.internal == nil {
		err := q.setInternal(atShutdown, func(data ...Data) []Data {
			for _, datum := range data {
				err := q.Push(datum)
				if err != nil && err != ErrAlreadyInQueue {
					log.Error("Unable push to channelled queue: %v", err)
				}
			}
			return nil
		}, q.channelQueue.exemplar)
		q.lock.Unlock()
		if err != nil {
			log.Fatal("Unable to create internal queue for %s Error: %v", q.Name(), err)
			return
		}
	} else {
		q.lock.Unlock()
	}
	atShutdown(q.Shutdown)
	atTerminate(q.Terminate)
	_ = q.channelQueue.AddWorkers(q.channelQueue.workers, 0)

	if luq, ok := q.internal.(*LevelUniqueQueue); ok && luq.ByteFIFOUniqueQueue.byteFIFO.Len(luq.shutdownCtx) != 0 {
		// Just run the level queue - we shut it down once it's flushed
		go q.internal.Run(func(_ func()) {}, func(_ func()) {})
		go func() {
			_ = q.internal.Flush(0)
			log.Debug("LevelUniqueQueue: %s flushed so shutting down", q.internal.(*LevelUniqueQueue).Name())
			q.internal.(*LevelUniqueQueue).Shutdown()
			GetManager().Remove(q.internal.(*LevelUniqueQueue).qid)
		}()
	} else {
		log.Debug("PersistableChannelUniqueQueue: %s Skipping running the empty level queue", q.delayedStarter.name)
		q.internal.(*LevelUniqueQueue).Shutdown()
		GetManager().Remove(q.internal.(*LevelUniqueQueue).qid)
	}

}

// Flush flushes the queue
func (q *PersistableChannelUniqueQueue) Flush(timeout time.Duration) error {
	return q.channelQueue.Flush(timeout)
}

// FlushWithContext flushes the queue
func (q *PersistableChannelUniqueQueue) FlushWithContext(ctx context.Context) error {
	return q.channelQueue.FlushWithContext(ctx)
}

// IsEmpty checks if a queue is empty
func (q *PersistableChannelUniqueQueue) IsEmpty() bool {
	return q.channelQueue.IsEmpty()
}

// Shutdown processing this queue
func (q *PersistableChannelUniqueQueue) Shutdown() {
	log.Trace("PersistableChannelUniqueQueue: %s Shutting down", q.delayedStarter.name)
	q.lock.Lock()
	select {
	case <-q.closed:
		q.lock.Unlock()
		return
	default:
		if q.internal != nil {
			q.internal.(*LevelUniqueQueue).Shutdown()
		}
		close(q.closed)
		q.lock.Unlock()
	}

	log.Trace("PersistableChannelUniqueQueue: %s Cancelling pools", q.delayedStarter.name)
	q.internal.(*LevelUniqueQueue).baseCtxCancel()
	q.channelQueue.baseCtxCancel()
	log.Trace("PersistableChannelUniqueQueue: %s Waiting til done", q.delayedStarter.name)
	q.channelQueue.Wait()
	q.internal.(*LevelUniqueQueue).Wait()
	// Redirect all remaining data in the chan to the internal channel
	go func() {
		log.Trace("PersistableChannelUniqueQueue: %s Redirecting remaining data", q.delayedStarter.name)
		for data := range q.channelQueue.dataChan {
			_ = q.internal.Push(data)
		}
		log.Trace("PersistableChannelUniqueQueue: %s Done Redirecting remaining data", q.delayedStarter.name)
	}()

	log.Debug("PersistableChannelUniqueQueue: %s Shutdown", q.delayedStarter.name)
}

// Terminate this queue and close the queue
func (q *PersistableChannelUniqueQueue) Terminate() {
	log.Trace("PersistableChannelUniqueQueue: %s Terminating", q.delayedStarter.name)
	q.Shutdown()
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal != nil {
		q.internal.(*LevelUniqueQueue).Terminate()
	}
	log.Debug("PersistableChannelUniqueQueue: %s Terminated", q.delayedStarter.name)
}

func init() {
	queuesMap[PersistableChannelUniqueQueueType] = NewPersistableChannelUniqueQueue
}
