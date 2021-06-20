// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package generate

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"time"

	"code.gitea.io/gitea/modules/util"
	"github.com/dgrijalva/jwt-go"
)

// NewInternalToken generate a new value intended to be used by INTERNAL_TOKEN.
func NewInternalToken() (string, error) {
	secretBytes := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, secretBytes)
	if err != nil {
		return "", err
	}

	secretKey := base64.RawURLEncoding.EncodeToString(secretBytes)

	now := time.Now()

	var internalToken string
	internalToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"nbf": now.Unix(),
	}).SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return internalToken, nil
}

// NewJwtSecret generates a new value intended to be used for JWT secrets.
func NewJwtSecret() ([]byte, error) {
	bytes := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// NewJwtSecretBase64 generates a new base64 encoded value intended to be used for JWT secrets.
func NewJwtSecretBase64() (string, error) {
	bytes, err := NewJwtSecret()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// NewSecretKey generate a new value intended to be used by SECRET_KEY.
func NewSecretKey() (string, error) {
	secretKey, err := util.RandomString(64)
	if err != nil {
		return "", err
	}

	return secretKey, nil
}
