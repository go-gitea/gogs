// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"testing"

	"code.gitea.io/gitea/modules/auth/oauth2"
)

func Test_TokenSign(t *testing.T) {

	scopes := ResolveScopeList("registry:catalog:* repository:library/busybox:pull,push")

	idToken := &ClaimSet{
		Access: scopes,
	}

	oauth2.InitSigningKey()

	signingKey := oauth2.DefaultSigningKey
	if signingKey.IsSymmetric() {
		t.FailNow()
	}

	_, err := idToken.SignToken(signingKey)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

}
