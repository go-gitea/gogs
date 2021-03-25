// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	ini "gopkg.in/ini.v1"
)

func Test_GetCronSettings(t *testing.T) {
	type BaseStruct struct {
		Base   bool
		Second string
	}

	type Extended struct {
		BaseStruct
		Extend bool
	}

	iniStr := `
[cron.test]
Base = true
Second = white rabbit
Extend = true
`
	Cfg, _ = ini.Load([]byte(iniStr))

	extended := &Extended{
		BaseStruct: BaseStruct{
			Second: "queen of hearts",
		},
	}

	_, err := GetCronSettings("test", extended)

	assert.NoError(t, err)
	assert.True(t, extended.Base)
	assert.EqualValues(t, extended.Second, "white rabbit")
	assert.True(t, extended.Extend)
}
