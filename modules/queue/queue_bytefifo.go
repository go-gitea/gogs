// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/goccy/go-json"
)

// ByteFIFOQueueConfiguration is the configuration for a ByteFIFOQueue
type ByteFIFOQueueConfiguration struct {
	WorkerPoolConfiguration
	Workers     int
	Name        string
	WaitOnEmpty bool
}

var _ Queue = &ByteFIFOQueue{}

// ByteFIFOQueue is a Queue formed from a ByteFIFO and WorkerPool
type ByteFIFOQueue struct {
	*WorkerPool
	byteFIFO           ByteFIFO
	typ                Type
	shutdownCtx        context.Context
	shutdownCtxCancel  context.CancelFunc
	terminateCtx       context.Context
	terminateCtxCancel context.CancelFunc
	exemplar           interface{}
	workers            int
	name               string
	lock               sync.Mutex
	waitOnEmpty        bool
	pushed             chan struct{}
}

// NewByteFIFOQueue creates a new ByteFIFOQueue
func NewByteFIFOQueue(typ Type, byteFIFO ByteFIFO, handle HandlerFunc, cfg, exemplar interface{}) (*ByteFIFOQueue, error) {
	configInterface, err := toConfig(ByteFIFOQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ByteFIFOQueueConfiguration)

	terminateCtx, terminateCtxCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCtxCancel := context.WithCancel(terminateCtx)

	return &ByteFIFOQueue{
		WorkerPool:         NewWorkerPool(handle, config.WorkerPoolConfiguration),
		byteFIFO:           byteFIFO,
		typ:                typ,
		shutdownCtx:        shutdownCtx,
		shutdownCtxCancel:  shutdownCtxCancel,
		terminateCtx:       terminateCtx,
		terminateCtxCancel: terminateCtxCancel,
		exemplar:           exemplar,
		workers:            config.Workers,
		name:               config.Name,
		waitOnEmpty:        config.WaitOnEmpty,
		pushed:             make(chan struct{}, 1),
	}, nil
}

// Name returns the name of this queue
func (q *ByteFIFOQueue) Name() string {
	return q.name
}

// Push pushes data to the fifo
func (q *ByteFIFOQueue) Push(data Data) error {
	return q.PushFunc(data, nil)
}

// PushFunc pushes data to the fifo
func (q *ByteFIFOQueue) PushFunc(data Data, fn func() error) error {
	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, q.exemplar, q.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if q.waitOnEmpty {
		defer func() {
			select {
			case q.pushed <- struct{}{}:
			default:
			}
		}()
	}
	return q.byteFIFO.PushFunc(q.terminateCtx, bs, fn)
}

// IsEmpty checks if the queue is empty
func (q *ByteFIFOQueue) IsEmpty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	if !q.WorkerPool.IsEmpty() {
		return false
	}
	return q.byteFIFO.Len(q.terminateCtx) == 0
}

// Run runs the bytefifo queue
func (q *ByteFIFOQueue) Run(atShutdown, atTerminate func(func())) {
	atShutdown(q.Shutdown)
	atTerminate(q.Terminate)
	log.Debug("%s: %s Starting", q.typ, q.name)

	_ = q.AddWorkers(q.workers, 0)

	log.Trace("%s: %s Now running", q.typ, q.name)
	q.readToChan()

	<-q.shutdownCtx.Done()
	log.Trace("%s: %s Waiting til done", q.typ, q.name)
	q.Wait()

	log.Trace("%s: %s Waiting til cleaned", q.typ, q.name)
	q.CleanUp(q.terminateCtx)
	q.terminateCtxCancel()
}

const maxBackOffTime = time.Second * 3

func (q *ByteFIFOQueue) readToChan() {
	// handle quick cancels
	select {
	case <-q.shutdownCtx.Done():
		// tell the pool to shutdown.
		q.baseCtxCancel()
		return
	default:
	}

	// Default backoff values
	backOffTime := time.Millisecond * 100

loop:
	for {
		err := q.doPop()
		if err == errQueueEmpty {
			log.Trace("%s: %s Waiting on Empty", q.typ, q.name)
			select {
			case <-q.pushed:
				// reset backOffTime
				backOffTime = 100 * time.Millisecond
				continue loop
			case <-q.shutdownCtx.Done():
				// Oops we've been shutdown whilst waiting
				// Make sure the worker pool is shutdown too
				q.baseCtxCancel()
				return
			}
		}

		// Reset the backOffTime if there is no error or an unmarshalError
		if err == nil || err == errUnmarshal {
			backOffTime = 100 * time.Millisecond
		}

		if err != nil {
			// Need to Backoff
			select {
			case <-q.shutdownCtx.Done():
				// Oops we've been shutdown whilst backing off
				// Make sure the worker pool is shutdown too
				q.baseCtxCancel()
				return
			case <-time.After(backOffTime):
				// OK we've waited - so backoff a bit
				backOffTime += backOffTime / 2
				if backOffTime > maxBackOffTime {
					backOffTime = maxBackOffTime
				}
				continue loop
			}
		}
		select {
		case <-q.shutdownCtx.Done():
			// Oops we've been shutdown
			// Make sure the worker pool is shutdown too
			q.baseCtxCancel()
			return
		default:
			continue loop
		}
	}
}

var errQueueEmpty = fmt.Errorf("empty queue")
var errEmptyBytes = fmt.Errorf("empty bytes")
var errUnmarshal = fmt.Errorf("failed to unmarshal")

func (q *ByteFIFOQueue) doPop() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	bs, err := q.byteFIFO.Pop(q.shutdownCtx)
	if err != nil {
		if err == context.Canceled {
			q.baseCtxCancel()
			return err
		}
		log.Error("%s: %s Error on Pop: %v", q.typ, q.name, err)
		return err
	}
	if len(bs) == 0 {
		if q.waitOnEmpty && q.byteFIFO.Len(q.shutdownCtx) == 0 {
			return errQueueEmpty
		}
		return errEmptyBytes
	}

	data, err := unmarshalAs(bs, q.exemplar)
	if err != nil {
		log.Error("%s: %s Failed to unmarshal with error: %v", q.typ, q.name, err)
		return errUnmarshal
	}

	log.Trace("%s %s: Task found: %#v", q.typ, q.name, data)
	q.WorkerPool.Push(data)
	return nil
}

// Shutdown processing from this queue
func (q *ByteFIFOQueue) Shutdown() {
	log.Trace("%s: %s Shutting down", q.typ, q.name)
	select {
	case <-q.shutdownCtx.Done():
		return
	default:
	}
	q.shutdownCtxCancel()
	log.Debug("%s: %s Shutdown", q.typ, q.name)
}

// IsShutdown returns a channel which is closed when this Queue is shutdown
func (q *ByteFIFOQueue) IsShutdown() <-chan struct{} {
	return q.shutdownCtx.Done()
}

// Terminate this queue and close the queue
func (q *ByteFIFOQueue) Terminate() {
	log.Trace("%s: %s Terminating", q.typ, q.name)
	q.Shutdown()
	select {
	case <-q.terminateCtx.Done():
		return
	default:
	}
	if log.IsDebug() {
		log.Debug("%s: %s Closing with %d tasks left in queue", q.typ, q.name, q.byteFIFO.Len(q.terminateCtx))
	}
	q.terminateCtxCancel()
	if err := q.byteFIFO.Close(); err != nil {
		log.Error("Error whilst closing internal byte fifo in %s: %s: %v", q.typ, q.name, err)
	}
	log.Debug("%s: %s Terminated", q.typ, q.name)
}

// IsTerminated returns a channel which is closed when this Queue is terminated
func (q *ByteFIFOQueue) IsTerminated() <-chan struct{} {
	return q.terminateCtx.Done()
}

var _ UniqueQueue = &ByteFIFOUniqueQueue{}

// ByteFIFOUniqueQueue represents a UniqueQueue formed from a UniqueByteFifo
type ByteFIFOUniqueQueue struct {
	ByteFIFOQueue
}

// NewByteFIFOUniqueQueue creates a new ByteFIFOUniqueQueue
func NewByteFIFOUniqueQueue(typ Type, byteFIFO UniqueByteFIFO, handle HandlerFunc, cfg, exemplar interface{}) (*ByteFIFOUniqueQueue, error) {
	configInterface, err := toConfig(ByteFIFOQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ByteFIFOQueueConfiguration)
	terminateCtx, terminateCtxCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdownCtxCancel := context.WithCancel(terminateCtx)

	return &ByteFIFOUniqueQueue{
		ByteFIFOQueue: ByteFIFOQueue{
			WorkerPool:         NewWorkerPool(handle, config.WorkerPoolConfiguration),
			byteFIFO:           byteFIFO,
			typ:                typ,
			shutdownCtx:        shutdownCtx,
			shutdownCtxCancel:  shutdownCtxCancel,
			terminateCtx:       terminateCtx,
			terminateCtxCancel: terminateCtxCancel,
			exemplar:           exemplar,
			workers:            config.Workers,
			name:               config.Name,
		},
	}, nil
}

// Has checks if the provided data is in the queue
func (q *ByteFIFOUniqueQueue) Has(data Data) (bool, error) {
	if !assignableTo(data, q.exemplar) {
		return false, fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, q.exemplar, q.name)
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	return q.byteFIFO.(UniqueByteFIFO).Has(q.terminateCtx, bs)
}
