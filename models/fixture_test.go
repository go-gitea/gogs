// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestFixtureGeneration(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(gen func() (string, error), name string) {
		expected, err := gen()
		if !assert.NoError(t, err) {
			return
		}
		bytes, err := ioutil.ReadFile(filepath.Join(fixturesDir, os.Getenv("TABLE_NAME_PREFIX")+name+".yml"))
		if !assert.NoError(t, err) {
			return
		}
		data := string(util.NormalizeEOL(bytes))
		assert.True(t, data == expected, "Differences detected for %s.yml", name)
	}

	test(GetYamlFixturesAccess, "access")
}
