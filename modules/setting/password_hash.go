// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/auth/hash"
	"code.gitea.io/gitea/modules/log"
)

func newPasswordHashService() {
	passwordHashAlgo := Cfg.Section("security").Key("PASSWORD_HASH_ALGO").MustString("pbkdf2")

	if _, ok := hash.DefaultHasher.Hashers[passwordHashAlgo]; !ok {
		log.Error("Unknown default hashing algorithm: %s. Keeping default: %s", passwordHashAlgo, hash.DefaultHasher.DefaultAlgorithm)
	} else {
		hash.DefaultHasher.DefaultAlgorithm = passwordHashAlgo
	}

	sec := Cfg.Section("security.password_hash")
	for _, hasher := range hash.DefaultHasher.Hashers {
		_ = sec.MapTo(hasher)
	}
}
