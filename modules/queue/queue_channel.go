// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// ChannelQueueType is the type for channel queue
const ChannelQueueType Type = "channel"

// ChannelQueueConfiguration is the configuration for a ChannelQueue
type ChannelQueueConfiguration struct {
	WorkerPoolConfiguration
	Workers int
	Name    string
}

// ChannelQueue implements Queue
//
// A channel queue is not persistable and does not shutdown or terminate cleanly
// It is basically a very thin wrapper around a WorkerPool
type ChannelQueue struct {
	*WorkerPool
	shutdownCtx        context.Context
	shutdownCtxCancel  context.CancelFunc
	terminateCtx       context.Context
	terminateCtxCancel context.CancelFunc
	exemplar           interface{}
	workers            int
	name               string
}

// NewChannelQueue creates a memory channel queue
func NewChannelQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(ChannelQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ChannelQueueConfiguration)
	if config.BatchLength == 0 {
		config.BatchLength = 1
	}

	terminateCtx, terminateCtxCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCtxCancel := context.WithCancel(terminateCtx)

	queue := &ChannelQueue{
		shutdownCtx:        shutdownCtx,
		shutdownCtxCancel:  shutdownCtxCancel,
		terminateCtx:       terminateCtx,
		terminateCtxCancel: terminateCtxCancel,
		exemplar:           exemplar,
		workers:            config.Workers,
		name:               config.Name,
	}
	queue.WorkerPool = NewWorkerPool(func(data ...Data) []Data {
		unhandled := handle(data...)
		if len(unhandled) > 0 {
			// We can only pushback to the channel if we're paused.
			if queue.IsPaused() {
				atomic.AddInt64(&queue.numInQueue, int64(len(unhandled)))
				go func() {
					for _, datum := range data {
						queue.dataChan <- datum
					}
				}()
				return nil
			}
		}
		return unhandled
	}, config.WorkerPoolConfiguration)

	queue.qid = GetManager().Add(queue, ChannelQueueType, config, exemplar)
	return queue, nil
}

// Run starts to run the queue
func (q *ChannelQueue) Run(atShutdown, atTerminate func(func())) {
	atShutdown(q.Shutdown)
	atTerminate(q.Terminate)
	log.Debug("ChannelQueue: %s Starting", q.name)
	_ = q.AddWorkers(q.workers, 0)
}

// Push will push data into the queue
func (q *ChannelQueue) Push(data Data) error {
	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in queue: %s", data, q.exemplar, q.name)
	}
	q.WorkerPool.Push(data)
	return nil
}

// Flush flushes the channel with a timeout - the Flush worker will be registered as a flush worker with the manager
func (q *ChannelQueue) Flush(timeout time.Duration) error {
	if q.IsPaused() {
		return nil
	}
	ctx, cancel := q.commonRegisterWorkers(1, timeout, true)
	defer cancel()
	return q.FlushWithContext(ctx)
}

// FlushWithContext is very similar to CleanUp but it will return as soon as the dataChan is empty
func (q *ChannelQueue) FlushWithContext(ctx context.Context) error {
	log.Trace("ChannelQueue: %d Flush", q.qid)
	paused, _ := q.IsPausedIsResumed()
	for {
		select {
		case <-paused:
			return nil
		case data := <-q.dataChan:
			if unhandled := q.handle(data); unhandled != nil {
				log.Error("Unhandled Data whilst flushing queue %d", q.qid)
			}
			atomic.AddInt64(&q.numInQueue, -1)
		case <-q.baseCtx.Done():
			return q.baseCtx.Err()
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
}

// Shutdown processing from this queue
func (q *ChannelQueue) Shutdown() {
	q.lock.Lock()
	defer q.lock.Unlock()
	select {
	case <-q.shutdownCtx.Done():
		log.Trace("ChannelQueue: %s Already Shutting down", q.name)
		return
	default:
	}
	log.Trace("ChannelQueue: %s Shutting down", q.name)
	go func() {
		log.Trace("ChannelQueue: %s Flushing", q.name)
		// We can't use Cleanup here because that will close the channel
		if err := q.FlushWithContext(q.terminateCtx); err != nil {
			log.Warn("ChannelQueue: %s Terminated before completed flushing", q.name)
			return
		}
		log.Debug("ChannelQueue: %s Flushed", q.name)
	}()
	q.shutdownCtxCancel()
	log.Debug("ChannelQueue: %s Shutdown", q.name)
}

// Terminate this queue and close the queue
func (q *ChannelQueue) Terminate() {
	log.Trace("ChannelQueue: %s Terminating", q.name)
	q.Shutdown()
	select {
	case <-q.terminateCtx.Done():
		return
	default:
	}
	q.terminateCtxCancel()
	log.Debug("ChannelQueue: %s Terminated", q.name)
}

// Name returns the name of this queue
func (q *ChannelQueue) Name() string {
	return q.name
}

func init() {
	queuesMap[ChannelQueueType] = NewChannelQueue
}
