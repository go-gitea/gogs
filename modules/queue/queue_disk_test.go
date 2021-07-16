// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
)

func TestLevelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) []Data {
		assert.True(t, len(data) == 2)
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
		return nil
	}

	var lock sync.Mutex
	queueShutdown := []func(){}
	queueTerminate := []func(){}

	tmpDir, err := ioutil.TempDir("", "level-queue-test-data")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	queue, err := NewLevelQueue(handle, LevelQueueConfiguration{
		ByteFIFOQueueConfiguration: ByteFIFOQueueConfiguration{
			WorkerPoolConfiguration: WorkerPoolConfiguration{
				QueueLength:  20,
				BatchLength:  2,
				BlockTimeout: 1 * time.Second,
				BoostTimeout: 5 * time.Minute,
				BoostWorkers: 5,
				MaxWorkers:   10,
			},
			Workers: 1,
		},
		DataDir: tmpDir,
	}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(shutdown func()) {
		lock.Lock()
		queueShutdown = append(queueShutdown, shutdown)
		lock.Unlock()
	}, func(terminate func()) {
		lock.Lock()
		queueTerminate = append(queueTerminate, terminate)
		lock.Unlock()
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

	err = queue.Push(test1)
	assert.Error(t, err)

	lock.Lock()
	for _, callback := range queueShutdown {
		callback()
	}
	lock.Unlock()

	time.Sleep(200 * time.Millisecond)
	err = queue.Push(&test1)
	assert.NoError(t, err)
	err = queue.Push(&test2)
	assert.NoError(t, err)
	select {
	case <-handleChan:
		assert.Fail(t, "Handler processing should have stopped")
	default:
	}
	lock.Lock()
	for _, callback := range queueTerminate {
		callback()
	}
	lock.Unlock()

	// Reopen queue
	queue, err = NewWrappedQueue(handle,
		WrappedQueueConfiguration{
			Underlying: LevelQueueType,
			Config: LevelQueueConfiguration{
				ByteFIFOQueueConfiguration: ByteFIFOQueueConfiguration{
					WorkerPoolConfiguration: WorkerPoolConfiguration{
						QueueLength:  20,
						BatchLength:  2,
						BlockTimeout: 1 * time.Second,
						BoostTimeout: 5 * time.Minute,
						BoostWorkers: 5,
						MaxWorkers:   10,
					},
					Workers: 1,
				},
				DataDir: tmpDir,
			},
		}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(shutdown func()) {
		lock.Lock()
		queueShutdown = append(queueShutdown, shutdown)
		lock.Unlock()
	}, func(terminate func()) {
		lock.Lock()
		queueTerminate = append(queueTerminate, terminate)
		lock.Unlock()
	})

	result3 := <-handleChan
	assert.Equal(t, test1.TestString, result3.TestString)
	assert.Equal(t, test1.TestInt, result3.TestInt)

	result4 := <-handleChan
	assert.Equal(t, test2.TestString, result4.TestString)
	assert.Equal(t, test2.TestInt, result4.TestInt)

	lock.Lock()
	for _, callback := range queueShutdown {
		callback()
	}
	for _, callback := range queueTerminate {
		callback()
	}
	lock.Unlock()
}
