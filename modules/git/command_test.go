// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build race

package git

import (
	"context"
	"testing"
	"time"
)

func TestRunInDirTimeoutPipelineNoTimeout(t *testing.T) {
	maxLoops := 1000

	// 'git --version' does not block so it must be finished before the timeout triggered.
	cmd := NewCommand("--version")
	for i := 0; i < maxLoops; i++ {
		if err := cmd.RunInDirTimeoutPipeline(-1, "", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunInDirTimeoutPipelineAlwaysTimeout(t *testing.T) {
	maxLoops := 1000

	// 'git hash-object --stdin' blocks on stdin so we can have the timeout triggered.
	cmd := NewCommand("hash-object", "--stdin")
	for i := 0; i < maxLoops; i++ {
		if err := cmd.RunInDirTimeoutPipeline(1*time.Microsecond, "", nil, nil); err != nil {
			if err != context.DeadlineExceeded {
				t.Fatalf("Testing %d/%d: %v", i, maxLoops, err)
			}
		}
	}
}
