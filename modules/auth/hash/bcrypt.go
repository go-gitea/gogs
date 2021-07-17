// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hash

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// BCryptHasher is a Hash implementation for BCrypt
type BCryptHasher struct {
	Cost     int `ini:"BCRYPT_COST"`
	fallback string
}

// HashPassword returns a PasswordHash, PassWordAlgo (and optionally an error)
func (h *BCryptHasher) HashPassword(password, salt, config string) (string, string, error) {
	if config == "fallback" {
		// Fixed default config to match with original configuration
		config = h.fallback
	} else if config == "" {
		config = h.getConfigFromSetting()
	}

	cost, err := strconv.Atoi(config)
	if err == nil {
		var tempPasswd []byte
		tempPasswd, _ = bcrypt.GenerateFromPassword([]byte(password), cost)
		return string(tempPasswd), fmt.Sprintf("bcrypt$%d", cost), nil
	}
	return "", "", err
}

// Validate validates a plain-text password
func (h *BCryptHasher) Validate(password, hash, salt, config string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (h *BCryptHasher) getConfigFromAlgo(algo string) string {
	split := strings.SplitN(algo, "$", 2)
	return split[1]
}

func (h *BCryptHasher) getConfigFromSetting() string {
	if h.Cost == 0 {
		return h.fallback
	}
	return strconv.Itoa(h.Cost)
}

func init() {
	DefaultHasher.Hashers["bcrypt"] = &BCryptHasher{10, "10"}
}
