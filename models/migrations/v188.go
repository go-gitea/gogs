// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addKeyIsVerified(x *xorm.Engine) error {
	type GPGKey struct {
		Verified bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(GPGKey))
}
