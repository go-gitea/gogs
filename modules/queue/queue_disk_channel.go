// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// PersistableChannelQueueType is the type for persistable queue
const PersistableChannelQueueType Type = "persistable-channel"

// PersistableChannelQueueConfiguration is the configuration for a PersistableChannelQueue
type PersistableChannelQueueConfiguration struct {
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

// PersistableChannelQueue wraps a channel queue and level queue together
// The disk level queue will be used to store data at shutdown and terminate - and will be restored
// on start up.
type PersistableChannelQueue struct {
	channelQueue *ChannelQueue
	delayedStarter
	lock   sync.Mutex
	closed chan struct{}
}

// NewPersistableChannelQueue creates a wrapped batched channel queue with persistable level queue backend when shutting down
// This differs from a wrapped queue in that the persistent queue is only used to persist at shutdown/terminate
func NewPersistableChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(PersistableChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(PersistableChannelQueueConfiguration)

	queue := &PersistableChannelQueue{
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

	channelQueue, err := NewChannelQueue(wrappedHandle, ChannelQueueConfiguration{
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
	levelCfg := LevelQueueConfiguration{
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

	levelQueue, err := NewLevelQueue(wrappedHandle, levelCfg, exemplar)
	if err == nil {
		queue.channelQueue = channelQueue.(*ChannelQueue)
		queue.delayedStarter = delayedStarter{
			internal: levelQueue.(*LevelQueue),
			name:     config.Name,
		}
		_ = GetManager().Add(queue, PersistableChannelQueueType, config, exemplar)
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	queue.channelQueue = channelQueue.(*ChannelQueue)
	queue.delayedStarter = delayedStarter{
		cfg:         levelCfg,
		underlying:  LevelQueueType,
		timeout:     config.Timeout,
		maxAttempts: config.MaxAttempts,
		name:        config.Name,
	}
	_ = GetManager().Add(queue, PersistableChannelQueueType, config, exemplar)
	return queue, nil
}

// Name returns the name of this queue
func (q *PersistableChannelQueue) Name() string {
	return q.delayedStarter.name
}

// Push will push the indexer data to queue
func (q *PersistableChannelQueue) Push(data Data) error {
	select {
	case <-q.closed:
		return q.internal.Push(data)
	default:
		return q.channelQueue.Push(data)
	}
}

// PushBack will push the indexer data to queue
func (q *PersistableChannelQueue) PushBack(data Data) error {
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

// Run starts to run the queue
func (q *PersistableChannelQueue) Run(atShutdown, atTerminate func(func())) {
	log.Debug("PersistableChannelQueue: %s Starting", q.delayedStarter.name)
	_ = q.channelQueue.AddWorkers(q.channelQueue.workers, 0)

	q.lock.Lock()
	if q.internal == nil {
		err := q.setInternal(atShutdown, q.channelQueue.handle, q.channelQueue.exemplar)
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

	if lq, ok := q.internal.(*LevelQueue); ok && lq.byteFIFO.Len(lq.shutdownCtx) != 0 {
		// Just run the level queue - we shut it down once it's flushed
		go q.internal.Run(func(_ func()) {}, func(_ func()) {})
		go func() {
			for !q.IsEmpty() {
				_ = q.internal.Flush(0)
				select {
				case <-time.After(100 * time.Millisecond):
				case <-q.internal.(*LevelQueue).shutdownCtx.Done():
					log.Warn("LevelQueue: %s shut down before completely flushed", q.internal.(*LevelQueue).Name())
					return
				}
			}
			log.Debug("LevelQueue: %s flushed so shutting down", q.internal.(*LevelQueue).Name())
			q.internal.(*LevelQueue).Shutdown()
			GetManager().Remove(q.internal.(*LevelQueue).qid)
		}()
	} else {
		log.Debug("PersistableChannelQueue: %s Skipping running the empty level queue", q.delayedStarter.name)
		q.internal.(*LevelQueue).Shutdown()
		GetManager().Remove(q.internal.(*LevelQueue).qid)
	}

}

// Flush flushes the queue and blocks till the queue is empty
func (q *PersistableChannelQueue) Flush(timeout time.Duration) error {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	return q.FlushWithContext(ctx)
}

// FlushWithContext flushes the queue and blocks till the queue is empty
func (q *PersistableChannelQueue) FlushWithContext(ctx context.Context) error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- q.channelQueue.FlushWithContext(ctx)
	}()
	go func() {
		q.lock.Lock()
		if q.internal == nil {
			q.lock.Unlock()
			errChan <- fmt.Errorf("not ready to flush internal queue %s yet", q.Name())
			return
		}
		q.lock.Unlock()
		errChan <- q.internal.FlushWithContext(ctx)
	}()
	err1 := <-errChan
	err2 := <-errChan

	if err1 != nil {
		return err1
	}
	return err2
}

// IsEmpty checks if a queue is empty
func (q *PersistableChannelQueue) IsEmpty() bool {
	if !q.channelQueue.IsEmpty() {
		return false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal == nil {
		return false
	}
	return q.internal.IsEmpty()
}

// IsPaused returns if the pool is paused
func (q *PersistableChannelQueue) IsPaused() bool {
	return q.channelQueue.IsPaused()
}

// IsPausedIsResumed returns if the pool is paused and a channel that is closed when it is resumed
func (q *PersistableChannelQueue) IsPausedIsResumed() (<-chan struct{}, <-chan struct{}) {
	return q.channelQueue.IsPausedIsResumed()
}

// Pause pauses the WorkerPool
func (q *PersistableChannelQueue) Pause() {
	q.channelQueue.Pause()
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal == nil {
		return
	}

	pausable, ok := q.internal.(Pausable)
	if !ok {
		return
	}
	pausable.Pause()
}

// Resume resumes the WorkerPool
func (q *PersistableChannelQueue) Resume() {
	q.channelQueue.Resume()
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.internal == nil {
		return
	}

	pausable, ok := q.internal.(Pausable)
	if !ok {
		return
	}
	pausable.Resume()
}

// Shutdown processing this queue
func (q *PersistableChannelQueue) Shutdown() {
	log.Trace("PersistableChannelQueue: %s Shutting down", q.delayedStarter.name)
	q.lock.Lock()

	select {
	case <-q.closed:
		q.lock.Unlock()
		return
	default:
	}
	q.channelQueue.Shutdown()
	if q.internal != nil {
		q.internal.(*LevelQueue).Shutdown()
	}
	close(q.closed)
	q.lock.Unlock()

	log.Trace("PersistableChannelQueue: %s Cancelling pools", q.delayedStarter.name)
	q.channelQueue.baseCtxCancel()
	q.internal.(*LevelQueue).baseCtxCancel()
	log.Trace("PersistableChannelQueue: %s Waiting til done", q.delayedStarter.name)
	q.channelQueue.Wait()
	q.internal.(*LevelQueue).Wait()
	// Redirect all remaining data in the chan to the internal channel
	go func() {
		log.Trace("PersistableChannelQueue: %s Redirecting remaining data", q.delayedStarter.name)
		for data := range q.channelQueue.dataChan {
			_ = q.internal.Push(data)
			atomic.AddInt64(&q.channelQueue.numInQueue, -1)
		}
		log.Trace("PersistableChannelQueue: %s Done Redirecting remaining data", q.delayedStarter.name)
	}()

	log.Debug("PersistableChannelQueue: %s Shutdown", q.delayedStarter.name)
}

// Terminate this queue and close the queue
func (q *PersistableChannelQueue) Terminate() {
	log.Trace("PersistableChannelQueue: %s Terminating", q.delayedStarter.name)
	q.Shutdown()
	q.lock.Lock()
	defer q.lock.Unlock()
	q.channelQueue.Terminate()
	if q.internal != nil {
		q.internal.(*LevelQueue).Terminate()
	}
	log.Debug("PersistableChannelQueue: %s Terminated", q.delayedStarter.name)
}

func init() {
	queuesMap[PersistableChannelQueueType] = NewPersistableChannelQueue
}
