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
	jsoniter "github.com/json-iterator/go"
)

// ByteFIFOQueueConfiguration is the configuration for a ByteFIFOQueue
type ByteFIFOQueueConfiguration struct {
	WorkerPoolConfiguration
	Workers int
	Name    string
}

var _ (Queue) = &ByteFIFOQueue{}

// ByteFIFOQueue is a Queue formed from a ByteFIFO and WorkerPool
type ByteFIFOQueue struct {
	*WorkerPool
	byteFIFO   ByteFIFO
	typ        Type
	closed     chan struct{}
	terminated chan struct{}
	exemplar   interface{}
	workers    int
	name       string
	paused     chan bool
	lock       sync.Mutex
}

// NewByteFIFOQueue creates a new ByteFIFOQueue
func NewByteFIFOQueue(typ Type, byteFIFO ByteFIFO, handle HandlerFunc, cfg, exemplar interface{}) (*ByteFIFOQueue, error) {
	configInterface, err := toConfig(ByteFIFOQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(ByteFIFOQueueConfiguration)

	return &ByteFIFOQueue{
		WorkerPool: NewWorkerPool(handle, config.WorkerPoolConfiguration),
		byteFIFO:   byteFIFO,
		typ:        typ,
		closed:     make(chan struct{}),
		terminated: make(chan struct{}),
		paused:     make(chan bool),
		exemplar:   exemplar,
		workers:    config.Workers,
		name:       config.Name,
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
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return q.byteFIFO.PushFunc(bs, fn)
}

// IsEmpty checks if the queue is empty
func (q *ByteFIFOQueue) IsEmpty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	if !q.WorkerPool.IsEmpty() {
		return false
	}
	return q.byteFIFO.Len() == 0
}

// Pause queue processing
func (q *ByteFIFOQueue) Pause() {
	q.paused <- true
}

// Resume queue processing
func (q *ByteFIFOQueue) Resume() {
	q.paused <- false
}

// Run runs the bytefifo queue
func (q *ByteFIFOQueue) Run(atShutdown, atTerminate func(context.Context, func())) {
	atShutdown(context.Background(), q.Shutdown)
	atTerminate(context.Background(), q.Terminate)
	log.Debug("%s: %s Starting", q.typ, q.name)

	go func() {
		_ = q.AddWorkers(q.workers, 0)
	}()

	go q.readToChan()

	log.Trace("%s: %s Waiting til closed", q.typ, q.name)
	<-q.closed
	log.Trace("%s: %s Waiting til done", q.typ, q.name)
	q.Wait()

	log.Trace("%s: %s Waiting til cleaned", q.typ, q.name)
	ctx, cancel := context.WithCancel(context.Background())
	atTerminate(ctx, cancel)
	q.CleanUp(ctx)
	cancel()
}

func (q *ByteFIFOQueue) readToChan() {
	paused := false
	for {
		if paused {
			select {
			case paused = <-q.paused:
				if paused {
					continue
				}
			}
		}
		select {
		case paused = <-q.paused:
		case <-q.closed:
			// tell the pool to shutdown.
			q.cancel()
			return
		default:
			q.lock.Lock()
			bs, err := q.byteFIFO.Pop()
			if err != nil {
				q.lock.Unlock()
				log.Error("%s: %s Error on Pop: %v", q.typ, q.name, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if len(bs) == 0 {
				q.lock.Unlock()
				time.Sleep(time.Millisecond * 100)
				continue
			}

			data, err := unmarshalAs(bs, q.exemplar)
			if err != nil {
				log.Error("%s: %s Failed to unmarshal with error: %v", q.typ, q.name, err)
				q.lock.Unlock()
				time.Sleep(time.Millisecond * 100)
				continue
			}

			log.Trace("%s %s: Task found: %#v", q.typ, q.name, data)
			q.WorkerPool.Push(data)
			q.lock.Unlock()
		}
	}
}

// Shutdown processing from this queue
func (q *ByteFIFOQueue) Shutdown() {
	log.Trace("%s: %s Shutting down", q.typ, q.name)
	q.lock.Lock()
	select {
	case <-q.closed:
	default:
		close(q.closed)
	}
	q.lock.Unlock()
	log.Debug("%s: %s Shutdown", q.typ, q.name)
}

// IsShutdown returns a channel which is closed when this Queue is shutdown
func (q *ByteFIFOQueue) IsShutdown() <-chan struct{} {
	return q.closed
}

// Terminate this queue and close the queue
func (q *ByteFIFOQueue) Terminate() {
	log.Trace("%s: %s Terminating", q.typ, q.name)
	q.Shutdown()
	q.lock.Lock()
	select {
	case <-q.terminated:
		q.lock.Unlock()
		return
	default:
	}
	close(q.terminated)
	q.lock.Unlock()
	if log.IsDebug() {
		log.Debug("%s: %s Closing with %d tasks left in queue", q.typ, q.name, q.byteFIFO.Len())
	}
	if err := q.byteFIFO.Close(); err != nil {
		log.Error("Error whilst closing internal byte fifo in %s: %s: %v", q.typ, q.name, err)
	}
	log.Debug("%s: %s Terminated", q.typ, q.name)
}

// IsTerminated returns a channel which is closed when this Queue is terminated
func (q *ByteFIFOQueue) IsTerminated() <-chan struct{} {
	return q.terminated
}

var _ (UniqueQueue) = &ByteFIFOUniqueQueue{}

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

	return &ByteFIFOUniqueQueue{
		ByteFIFOQueue: ByteFIFOQueue{
			WorkerPool: NewWorkerPool(handle, config.WorkerPoolConfiguration),
			byteFIFO:   byteFIFO,
			typ:        typ,
			closed:     make(chan struct{}),
			terminated: make(chan struct{}),
			paused:     make(chan bool),
			exemplar:   exemplar,
			workers:    config.Workers,
			name:       config.Name,
		},
	}, nil
}

// Has checks if the provided data is in the queue
func (q *ByteFIFOUniqueQueue) Has(data Data) (bool, error) {
	if !assignableTo(data, q.exemplar) {
		return false, fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, q.exemplar, q.name)
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	bs, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	return q.byteFIFO.(UniqueByteFIFO).Has(bs)
}
