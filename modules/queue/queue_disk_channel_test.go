// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
)

func TestPersistableChannelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		assert.True(t, len(data) == 2)
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	queueShutdown := []func(){}
	queueTerminate := []func(){}

	tmpDir, err := ioutil.TempDir("", "persistable-channel-queue-test-data")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	queue, err := NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  2,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "first",
	}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		queueTerminate = append(queueTerminate, terminate)
	})

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}

	err = queue.Push(&test1)
	assert.NoError(t, err)
	go func() {
		err := queue.Push(&test2)
		assert.NoError(t, err)
	}()

	result1 := <-handleChan
	assert.Equal(t, test1.TestString, result1.TestString)
	assert.Equal(t, test1.TestInt, result1.TestInt)

	result2 := <-handleChan
	assert.Equal(t, test2.TestString, result2.TestString)
	assert.Equal(t, test2.TestInt, result2.TestInt)

	// test1 is a testData not a *testData so will be rejected
	err = queue.Push(test1)
	assert.Error(t, err)

	// Now shutdown the queue
	for _, callback := range queueShutdown {
		callback()
	}

	// Wait til it is closed
	<-queue.(*PersistableChannelQueue).closed

	err = queue.Push(&test1)
	assert.NoError(t, err)
	err = queue.Push(&test2)
	assert.NoError(t, err)
	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	// terminate the queue
	for _, callback := range queueTerminate {
		callback()
	}

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	// Reopen queue
	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  2,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "second",
	}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		queueTerminate = append(queueTerminate, terminate)
	})

	result3 := <-handleChan
	assert.Equal(t, test1.TestString, result3.TestString)
	assert.Equal(t, test1.TestInt, result3.TestInt)

	result4 := <-handleChan
	assert.Equal(t, test2.TestString, result4.TestString)
	assert.Equal(t, test2.TestInt, result4.TestInt)
	for _, callback := range queueShutdown {
		callback()
	}
	for _, callback := range queueTerminate {
		callback()
	}

}

func TestPersistableChannelQueue_Pause(t *testing.T) {
	lock := sync.Mutex{}
	var queue Queue
	var err error
	pushBack := false

	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		lock.Lock()
		if pushBack {
			if pausable, ok := queue.(Pausable); ok {
				log.Info("pausing")
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

	queueShutdown := []func(){}
	queueTerminate := []func(){}

	tmpDir, err := ioutil.TempDir("", "persistable-channel-queue-pause-test-data")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  2,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "first",
	}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		queueTerminate = append(queueTerminate, terminate)
	})

	test1 := testData{"A", 1}
	test2 := testData{"B", 2}

	err = queue.Push(&test1)
	assert.NoError(t, err)

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

	// Now shutdown the queue
	for _, callback := range queueShutdown {
		callback()
	}

	// Wait til it is closed
	<-queue.(*PersistableChannelQueue).closed

	err = queue.Push(&test1)
	assert.NoError(t, err)
	err = queue.Push(&test2)
	assert.NoError(t, err)
	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	// terminate the queue
	for _, callback := range queueTerminate {
		callback()
	}

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	lock.Lock()
	pushBack = true
	lock.Unlock()

	// Reopen queue
	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:      tmpDir,
		BatchLength:  1,
		QueueLength:  20,
		Workers:      1,
		BoostWorkers: 0,
		MaxWorkers:   10,
		Name:         "second",
	}, &testData{})
	assert.NoError(t, err)
	pausable, ok = queue.(Pausable)
	if !assert.True(t, ok) {
		return
	}

	paused, _ = pausable.IsPausedIsResumed()

	go queue.Run(func(shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(terminate func()) {
		queueTerminate = append(queueTerminate, terminate)
	})

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	case <-paused:
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

	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}

	pausable.Resume()

	result3 := <-handleChan
	result4 := <-handleChan
	if result4.TestString == test1.TestString {
		result3, result4 = result4, result3
	}
	assert.Equal(t, test1.TestString, result3.TestString)
	assert.Equal(t, test1.TestInt, result3.TestInt)

	assert.Equal(t, test2.TestString, result4.TestString)
	assert.Equal(t, test2.TestInt, result4.TestInt)
	for _, callback := range queueShutdown {
		callback()
	}
	for _, callback := range queueTerminate {
		callback()
	}

}
