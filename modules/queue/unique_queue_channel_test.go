// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannelUniqueQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	nilFn := func(_ func()) {}

	queue, err := NewChannelUniqueQueue(handle,
		ChannelQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  0,
				MaxWorkers:   10,
				BlockTimeout: 1 * time.Second,
				BoostTimeout: 5 * time.Minute,
				BoostWorkers: 5,
			},
			Workers: 0,
			Name:    "TestChannelQueue",
		}, &testData{})
	assert.NoError(t, err)

	assert.Equal(t, queue.(*ChannelUniqueQueue).WorkerPool.boostWorkers, 5)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	go queue.Push(&test1)
	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	err = queue.Push(test1)
	assert.Error(t, err)
}

func TestChannelUniqueQueue_Batch(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	nilFn := func(_ func()) {}

	queue, err := NewChannelUniqueQueue(handle,
		ChannelQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  20,
				BatchLength:  2,
				BlockTimeout: 0,
				BoostTimeout: 0,
				BoostWorkers: 0,
				MaxWorkers:   10,
			},
			Workers: 1,
		}, &testData{})
	assert.NoError(t, err)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}

	queue.Push(&test1)
	go queue.Push(&test2)

	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	result2 := <-handleChan
	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	err = queue.Push(test1)
	assert.Error(t, err)
}

func TestChannelUniqueQueue_Pause(t *testing.T) {
	lock := sync.Mutex{}
	var queue Queue
	var err error
	pushBack := false
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		lock.Lock()
		if pushBack {
			if pausable, ok := queue.(Pausable); ok {
				pausable.Pause()
			}
			pushBack = false
			lock.Unlock()
			return data
		}
		lock.Unlock()

		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}
	nilFn := func(_ func()) {}

	queue, err = NewChannelUniqueQueue(handle,
		ChannelQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  20,
				BatchLength:  1,
				BlockTimeout: 0,
				BoostTimeout: 0,
				BoostWorkers: 0,
				MaxWorkers:   10,
			},
			Workers: 1,
		}, &testData{})
	assert.NoError(t, err)

	go queue.Run(nilFn, nilFn)

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}
	queue.Push(&test1)

	pausable, ok := queue.(Pausable)
	if !assert.True(t, ok) {
		return
	}
	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	pausable.Pause()

	paused, resumed := pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-resumed:
		assert.Fail(t, "Queue should not be resumed")
		return
	default:
		assert.Fail(t, "Queue is not paused")
		return
	}

	queue.Push(&test2)

	var result2 *testData
	select {
	case result2 = <-handleChan:
		assert.Fail(t, "handler chan should be empty")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Nil(t, result2)

	pausable.Resume()

	select {
	case <-resumed:
	default:
		assert.Fail(t, "Queue should be resumed")
	}

	select {
	case result2 = <-handleChan:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "handler chan should contain test2")
	}

	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	lock.Lock()
	pushBack = true
	lock.Unlock()

	paused, resumed = pausable.IsPausedIsResumed()

	select {
	case <-paused:
		assert.Fail(t, "Queue should not be paused")
		return
	case <-resumed:
	default:
		assert.Fail(t, "Queue is not resumed")
		return
	}

	queue.Push(&test1)

	select {
	case <-paused:
	case <-handleChan:
		assert.Fail(t, "handler chan should not contain test1")
		return
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "queue should be paused")
		return
	}

	paused, resumed = pausable.IsPausedIsResumed()

	select {
	case <-paused:
	case <-resumed:
		assert.Fail(t, "Queue should not be resumed")
		return
	default:
		assert.Fail(t, "Queue is not paused")
		return
	}

	pausable.Resume()

	select {
	case <-resumed:
	default:
		assert.Fail(t, "Queue should be resumed")
	}

	select {
	case result1 = <-handleChan:
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "handler chan should contain test1")
	}
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

}
