// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetManager(t *testing.T) {
	go func() {
		// test race protection
		_ = GetManager()
	}()
	pm := GetManager()
	assert.NotNil(t, pm)
}

func TestManager_Add(t *testing.T) {
	pm := Manager{processes: make(map[int64]*Process)}

	pid := pm.Add("foo", nil)
	assert.Equal(t, int64(1), pid, "expected to get pid 1 got %d", pid)

	pid = pm.Add("bar", nil)
	assert.Equal(t, int64(2), pid, "expected to get pid 2 got %d", pid)
}

func TestManager_Cancel(t *testing.T) {
	pm := Manager{processes: make(map[int64]*Process)}

	ctx, cancel := context.WithCancel(context.Background())
	pid := pm.Add("foo", cancel)

	pm.Cancel(pid)

	select {
	case <-ctx.Done():
	default:
		assert.Fail(t, "Cancel should cancel the provided context")
	}
}

func TestManager_Remove(t *testing.T) {
	pm := Manager{processes: make(map[int64]*Process)}

	pid1 := pm.Add("foo", nil)
	assert.Equal(t, int64(1), pid1, "expected to get pid 1 got %d", pid1)

	pid2 := pm.Add("bar", nil)
	assert.Equal(t, int64(2), pid2, "expected to get pid 2 got %d", pid2)

	pm.Remove(pid2)

	_, exists := pm.processes[pid2]
	assert.False(t, exists, "PID %d is in the list but shouldn't", pid2)
}

func TestExecTimeoutNever(t *testing.T) {
	// TODO Investigate how to improve the time elapsed per round.
	maxLoops := 10
	for i := 1; i < maxLoops; i++ {
		_, stderr, err := GetManager().ExecTimeout(5*time.Second, "ExecTimeout", "git", "--version")
		if err != nil {
			t.Fatalf("git --version: %v(%s)", err, stderr)
		}
	}
}

func TestExecTimeoutAlways(t *testing.T) {
	maxLoops := 100
	for i := 1; i < maxLoops; i++ {
		_, stderr, err := GetManager().ExecTimeout(100*time.Microsecond, "ExecTimeout", "sleep", "5")
		// TODO Simplify logging and errors to get precise error type. E.g. checking "if err != context.DeadlineExceeded".
		if err == nil {
			t.Fatalf("sleep 5 secs: %v(%s)", err, stderr)
		}
	}
}
