// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/log"
)

func fatalTestError(fmtStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

func TestMain(m *testing.M) {
	_ = log.NewLogger(1000, "console", "console", `{"level":"trace","stacktracelevel":"NONE","stderr":true}`)

	if err := Init(context.Background()); err != nil {
		fatalTestError("Init failed: %v", err)
	}

	exitStatus := m.Run()
	os.Exit(exitStatus)
}
