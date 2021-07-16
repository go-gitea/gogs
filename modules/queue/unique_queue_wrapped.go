// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"fmt"
	"sync"
	"time"
)

// WrappedUniqueQueueType is the type for a wrapped delayed starting queue
const WrappedUniqueQueueType Type = "unique-wrapped"

// WrappedUniqueQueueConfiguration is the configuration for a WrappedUniqueQueue
type WrappedUniqueQueueConfiguration struct {
	Underlying  Type
	Timeout     time.Duration
	MaxAttempts int
	Config      interface{}
	QueueLength int
	Name        string
}

// WrappedUniqueQueue wraps a delayed starting unique queue
type WrappedUniqueQueue struct {
	*WrappedQueue
	table map[Data]bool
	tlock sync.Mutex
	ready bool
}

// NewWrappedUniqueQueue will attempt to create a unique queue of the provided type,
// but if there is a problem creating this queue it will instead create
// a WrappedUniqueQueue with delayed startup of the queue instead and a
// channel which will be redirected to the queue
//
// Please note that this Queue does not guarantee that a particular
// task cannot be processed twice or more at the same time. Uniqueness is
// only guaranteed whilst the task is waiting in the queue.
func NewWrappedUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(WrappedUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(WrappedUniqueQueueConfiguration)

	queue, err := NewQueue(config.Underlying, handle, config.Config, exemplar)
	if err == nil {
		// Just return the queue there is no need to wrap
		return queue, nil
	}
	if IsErrInvalidConfiguration(err) {
		// Retrying ain't gonna make this any better...
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	wrapped := &WrappedUniqueQueue{
		WrappedQueue: &WrappedQueue{
			channel:  make(chan Data, config.QueueLength),
			exemplar: exemplar,
			delayedStarter: delayedStarter{
				cfg:         config.Config,
				underlying:  config.Underlying,
				timeout:     config.Timeout,
				maxAttempts: config.MaxAttempts,
				name:        config.Name,
			},
		},
		table: map[Data]bool{},
	}

	// wrapped.handle is passed to the delayedStarting internal queue and is run to handle
	// data passed to
	wrapped.handle = func(data ...Data) (unhandled []Data) {
		for _, datum := range data {
			wrapped.tlock.Lock()
			if !wrapped.ready {
				delete(wrapped.table, data)
				// If our table is empty all of the requests we have buffered between the
				// wrapper queue starting and the internal queue starting have been handled.
				// We can stop buffering requests in our local table and just pass Push
				// direct to the internal queue
				if len(wrapped.table) == 0 {
					wrapped.ready = true
				}
			}
			wrapped.tlock.Unlock()
			if u := handle(datum); u != nil {
				unhandled = append(unhandled, u...)
			}
		}
		return unhandled
	}
	_ = GetManager().Add(queue, WrappedUniqueQueueType, config, exemplar)
	return wrapped, nil
}

// Push will push the data to the internal channel checking it against the exemplar
func (q *WrappedUniqueQueue) Push(data Data) error {
	return q.PushFunc(data, nil)
}

// PushFunc will push the data to the internal channel checking it against the exemplar
func (q *WrappedUniqueQueue) PushFunc(data Data, fn func() error) error {
	if !assignableTo(data, q.exemplar) {
		return fmt.Errorf("Unable to assign data: %v to same type as exemplar: %v in %s", data, q.exemplar, q.name)
	}

	q.tlock.Lock()
	if q.ready {
		// ready means our table is empty and all of the requests we have buffered between the
		// wrapper queue starting and the internal queue starting have been handled.
		// We can stop buffering requests in our local table and just pass Push
		// direct to the internal queue
		q.tlock.Unlock()
		return q.internal.(UniqueQueue).PushFunc(data, fn)
	}

	locked := true
	defer func() {
		if locked {
			q.tlock.Unlock()
		}
	}()
	if _, ok := q.table[data]; ok {
		return ErrAlreadyInQueue
	}
	// FIXME: We probably need to implement some sort of limit here
	// If the downstream queue blocks this table will grow without limit
	q.table[data] = true
	if fn != nil {
		err := fn()
		if err != nil {
			delete(q.table, data)
			return err
		}
	}
	locked = false
	q.tlock.Unlock()

	q.channel <- data
	return nil
}

// Has checks if the data is in the queue
func (q *WrappedUniqueQueue) Has(data Data) (bool, error) {
	q.tlock.Lock()
	defer q.tlock.Unlock()
	if q.ready {
		return q.internal.(UniqueQueue).Has(data)
	}
	_, has := q.table[data]
	return has, nil
}

// IsEmpty checks whether the queue is empty
func (q *WrappedUniqueQueue) IsEmpty() bool {
	q.tlock.Lock()
	if len(q.table) > 0 {
		q.tlock.Unlock()
		return false
	}
	if q.ready {
		q.tlock.Unlock()
		return q.internal.IsEmpty()
	}
	q.tlock.Unlock()
	return false
}

func init() {
	queuesMap[WrappedUniqueQueueType] = NewWrappedUniqueQueue
}
