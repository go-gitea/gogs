// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

// LoginSource represents an external way for authorizing users.
type LoginSourceOriginalV189 struct {
	ID        int64 `xorm:"pk autoincr"`
	Type      int
	IsActived bool   `xorm:"INDEX NOT NULL DEFAULT false"`
	Cfg       string `xorm:"TEXT"`
	Expected  string `xorm:"TEXT"`
}

func (ls *LoginSourceOriginalV189) TableName() string {
	return "login_source"
}

func Test_unwrapLDAPSourceCfg(t *testing.T) {

	// Prepare and load the testing database
	x, deferable := prepareTestEnv(t, 0, new(LoginSourceOriginalV189))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	// LoginSource represents an external way for authorizing users.
	type LoginSource struct {
		ID       int64 `xorm:"pk autoincr"`
		Type     int
		IsActive bool   `xorm:"INDEX NOT NULL DEFAULT false"`
		Cfg      string `xorm:"TEXT"`
		Expected string `xorm:"TEXT"`
	}

	// Run the migration
	if err := unwrapLDAPSourceCfg(x); err != nil {
		assert.NoError(t, err)
		return
	}

	const batchSize = 100
	for start := 0; ; start += batchSize {
		sources := make([]*LoginSource, 0, batchSize)
		if err := x.Table("login_source").Limit(batchSize, start).Find(&sources); err != nil {
			assert.NoError(t, err)
			return
		}

		if len(sources) == 0 {
			break
		}

		for _, source := range sources {
			converted := map[string]interface{}{}
			expected := map[string]interface{}{}

			if err := jsoniter.Unmarshal([]byte(source.Cfg), &converted); err != nil {
				assert.NoError(t, err)
				return
			}

			if err := jsoniter.Unmarshal([]byte(source.Expected), &expected); err != nil {
				assert.NoError(t, err)
				return
			}

			assert.EqualValues(t, expected, converted, "unwrapLDAPSourceCfg failed for %d", source.ID)
			assert.EqualValues(t, source.ID%2 == 0, source.IsActive, "unwrapLDAPSourceCfg failed for %d", source.ID)
		}
	}

}
