// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"fmt"
	"time"
)

// ErrInvalidConfiguration is called when there is invalid configuration for a queue
type ErrInvalidConfiguration struct {
	cfg interface{}
	err error
}

func (err ErrInvalidConfiguration) Error() string {
	if err.err != nil {
		return fmt.Sprintf("Invalid Configuration Argument: %v: Error: %v", err.cfg, err.err)
	}
	return fmt.Sprintf("Invalid Configuration Argument: %v", err.cfg)
}

// IsErrInvalidConfiguration checks if an error is an ErrInvalidConfiguration
func IsErrInvalidConfiguration(err error) bool {
	_, ok := err.(ErrInvalidConfiguration)
	return ok
}

// Type is a type of Queue
type Type string

// Data defines an type of queuable data
type Data interface{}

// HandlerFunc is a function that takes a variable amount of data and processes it
type HandlerFunc func(...Data) (unhandled []Data)

// NewQueueFunc is a function that creates a queue
type NewQueueFunc func(handler HandlerFunc, config interface{}, exemplar interface{}) (Queue, error)

// Shutdownable represents a queue that can be shutdown
type Shutdownable interface {
	Shutdown()
	Terminate()
}

// Named represents a queue with a name
type Named interface {
	Name() string
}

// Queue defines an interface of a queue-like item
//
// Queues will handle their own contents in the Run method
type Queue interface {
	Flushable
	Run(atShutdown, atTerminate func(func()))
	Push(Data) error
}

// PushBackable queues can be pushed back to
type PushBackable interface {
	// PushBack pushes data back to the top of the fifo
	PushBack(Data) error
}

// DummyQueueType is the type for the dummy queue
const DummyQueueType Type = "dummy"

// NewDummyQueue creates a new DummyQueue
func NewDummyQueue(handler HandlerFunc, opts, exemplar interface{}) (Queue, error) {
	return &DummyQueue{}, nil
}

// DummyQueue represents an empty queue
type DummyQueue struct {
}

// Run does nothing
func (*DummyQueue) Run(_, _ func(func())) {}

// Push fakes a push of data to the queue
func (*DummyQueue) Push(Data) error {
	return nil
}

// PushFunc fakes a push of data to the queue with a function. The function is never run.
func (*DummyQueue) PushFunc(Data, func() error) error {
	return nil
}

// Has always returns false as this queue never does anything
func (*DummyQueue) Has(Data) (bool, error) {
	return false, nil
}

// Flush always returns nil
func (*DummyQueue) Flush(time.Duration) error {
	return nil
}

// FlushWithContext always returns nil
func (*DummyQueue) FlushWithContext(context.Context) error {
	return nil
}

// IsEmpty asserts that the queue is empty
func (*DummyQueue) IsEmpty() bool {
	return true
}

// ImmediateType is the type to execute the function when push
const ImmediateType Type = "immediate"

// NewImmediate creates a new false queue to execute the function when push
func NewImmediate(handler HandlerFunc, opts, exemplar interface{}) (Queue, error) {
	return &Immediate{
		handler: handler,
	}, nil
}

// Immediate represents an direct execution queue
type Immediate struct {
	handler HandlerFunc
}

// Run does nothing
func (*Immediate) Run(_, _ func(func())) {}

// Push fakes a push of data to the queue
func (q *Immediate) Push(data Data) error {
	return q.PushFunc(data, nil)
}

// PushFunc fakes a push of data to the queue with a function. The function is never run.
func (q *Immediate) PushFunc(data Data, f func() error) error {
	if f != nil {
		if err := f(); err != nil {
			return err
		}
	}
	q.handler(data)
	return nil
}

// Has always returns false as this queue never does anything
func (*Immediate) Has(Data) (bool, error) {
	return false, nil
}

// Flush always returns nil
func (*Immediate) Flush(time.Duration) error {
	return nil
}

// FlushWithContext always returns nil
func (*Immediate) FlushWithContext(context.Context) error {
	return nil
}

// IsEmpty asserts that the queue is empty
func (*Immediate) IsEmpty() bool {
	return true
}

var queuesMap = map[Type]NewQueueFunc{
	DummyQueueType: NewDummyQueue,
	ImmediateType:  NewImmediate,
}

// RegisteredTypes provides the list of requested types of queues
func RegisteredTypes() []Type {
	types := make([]Type, len(queuesMap))
	i := 0
	for key := range queuesMap {
		types[i] = key
		i++
	}
	return types
}

// RegisteredTypesAsString provides the list of requested types of queues
func RegisteredTypesAsString() []string {
	types := make([]string, len(queuesMap))
	i := 0
	for key := range queuesMap {
		types[i] = string(key)
		i++
	}
	return types
}

// NewQueue takes a queue Type, HandlerFunc, some options and possibly an exemplar and returns a Queue or an error
func NewQueue(queueType Type, handlerFunc HandlerFunc, opts, exemplar interface{}) (Queue, error) {
	newFn, ok := queuesMap[queueType]
	if !ok {
		return nil, fmt.Errorf("Unsupported queue type: %v", queueType)
	}
	return newFn(handlerFunc, opts, exemplar)
}
