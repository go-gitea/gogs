// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hash

import (
	"crypto/subtle"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// SCryptHasher is a Hash implementation for SCrypt
type SCryptHasher struct {
	N         int `ini:"SCRYPT_N"`
	R         int `ini:"SCRYPT_R"`
	P         int `ini:"SCRYPT_P"`
	KeyLength int `ini:"SCRYPT_KEY_LENGTH"`
	fallback  string
}

// HashPassword returns a PasswordHash, PassWordAlgo (and optionally an error)
func (h *SCryptHasher) HashPassword(password, salt, config string) (string, string, error) {
	var tempPasswd []byte
	if config == "fallback" {
		// Fixed default config to match with original configuration
		config = h.fallback
	}

	split := strings.Split(config, "$")
	if len(split) != 4 {
		split = strings.Split(h.getConfigFromSetting(), "$")
	}

	var n, r, p, keyLength int
	var err error

	if n, err = strconv.Atoi(split[0]); err != nil {
		return "", "", err
	}
	if r, err = strconv.Atoi(split[1]); err != nil {
		return "", "", err
	}
	if p, err = strconv.Atoi(split[2]); err != nil {
		return "", "", err
	}
	if keyLength, err = strconv.Atoi(split[3]); err != nil {
		return "", "", err
	}

	tempPasswd, _ = scrypt.Key([]byte(password), []byte(salt), n, r, p, keyLength)
	return fmt.Sprintf("%x", tempPasswd),
		fmt.Sprintf("scrypt$%d$%d$%d$%d", n, r, p, keyLength),
		nil
}

// Validate validates a plain-text password
func (h *SCryptHasher) Validate(password, hash, salt, config string) bool {
	tempHash, _, _ := h.HashPassword(password, salt, config)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(tempHash)) == 1
}

func (h *SCryptHasher) getConfigFromAlgo(algo string) string {
	split := strings.SplitN(algo, "$", 2)
	if len(split) == 1 {
		split[1] = "fallback"
	}
	return split[1]
}

func (h *SCryptHasher) getConfigFromSetting() string {
	if h.N == 0 || h.R == 0 || h.P == 0 || h.KeyLength == 0 {
		return h.fallback
	}
	return fmt.Sprintf("%d$%d$%d$%d", h.N, h.R, h.P, h.KeyLength)
}

func init() {
	DefaultHasher.Hashers["scrypt"] = &SCryptHasher{65536, 16, 2, 50, "65536$16$2$50"}
}
