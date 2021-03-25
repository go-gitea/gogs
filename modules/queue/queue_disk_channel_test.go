// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
)

func TestPersistableChannelQueue(t *testing.T) {
	handleChan := make(chan *testData)
	handle := func(data ...Data) {
		assert.True(t, len(data) == 2)
		for _, datum := range data {
			testDatum := datum.(*testData)
			handleChan <- testDatum
		}
	}

	queueShutdown := []func(){}
	queueTerminate := []func(){}

	tmpDir, err := ioutil.TempDir("", "persistable-channel-queue-test-data")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	queue, err := NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:     tmpDir,
		BatchLength: 2,
		QueueLength: 20,
		Workers:     1,
		MaxWorkers:  10,
	}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(_ context.Context, shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(_ context.Context, terminate func()) {
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

	err = queue.Push(test1)
	assert.Error(t, err)

	for _, callback := range queueShutdown {
		callback()
	}
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
	for _, callback := range queueTerminate {
		callback()
	}

	// Reopen queue
	queue, err = NewPersistableChannelQueue(handle, PersistableChannelQueueConfiguration{
		DataDir:     tmpDir,
		BatchLength: 2,
		QueueLength: 20,
		Workers:     1,
		MaxWorkers:  10,
	}, &testData{})
	assert.NoError(t, err)

	go queue.Run(func(_ context.Context, shutdown func()) {
		queueShutdown = append(queueShutdown, shutdown)
	}, func(_ context.Context, terminate func()) {
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
