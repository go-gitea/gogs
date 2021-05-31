// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/dgrijalva/jwt-go"
)

func Test_TokenIssuer(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatal(err)
	}

	scopes := ResolveScopeList("registry:catalog:* repository:library/busybox:pull,push")

	issuer := &TokenIssuer{
		Issuer:     "gitea",
		Audience:   "gitea-token-service",
		SigningKey: privKey,
		Expiration: 60,
	}

	jwtToken, err := issuer.CreateJWT("admin", scopes)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	_, err = jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		return &privKey.PublicKey, nil
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}
