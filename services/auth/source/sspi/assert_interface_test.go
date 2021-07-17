// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sspi_test

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/auth/source/sspi"
)

// This test file exists to assert that our Source exposes the interfaces that we expect
// It tightly binds the interfaces and implementation without breaking go import cycles

type sourceInterface interface {
	models.LoginConfig
}

var _ (sourceInterface) = &sspi.Source{}
