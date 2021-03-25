// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func createTestCache() {
	conn, _ = newCache(setting.Cache{
		Adapter: "memory",
		TTL:     time.Minute,
	})
}

func TestNewContext(t *testing.T) {
	assert.NoError(t, NewContext())

	setting.CacheService.Cache = setting.Cache{Enabled: true, Adapter: "redis", Conn: "some random string"}
	con, err := newCache(setting.Cache{
		Adapter:  "rand",
		Conn:     "false conf",
		Interval: 100,
	})
	assert.Error(t, err)
	assert.Nil(t, con)
}

func TestGetCache(t *testing.T) {
	createTestCache()

	assert.NotNil(t, GetCache())
}

func TestGetString(t *testing.T) {
	createTestCache()

	data, err := GetString("key", func() (string, error) {
		return "", fmt.Errorf("some error")
	})
	assert.Error(t, err)
	assert.Equal(t, "", data)

	data, err = GetString("key", func() (string, error) {
		return "", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "", data)

	// data, err = GetString("key", func() (string, error) {
	// 	return "some data", nil
	// })
	// assert.NoError(t, err)
	// assert.Equal(t, "", data)
	// Remove("key")

	data, err = GetString("key", func() (string, error) {
		return "some data", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "some data", data)

	// data, err = GetString("key", func() (string, error) {
	// 	return "", fmt.Errorf("some error")
	// })
	// assert.NoError(t, err)
	// assert.Equal(t, "some data", data)

	// TODO: uncommented code works in IDE but not with go test
}

func TestGetInt(t *testing.T) {
	createTestCache()

	data, err := GetInt("key", func() (int, error) {
		return 0, fmt.Errorf("some error")
	})
	assert.Error(t, err)
	assert.Equal(t, 0, data)

	data, err = GetInt("key", func() (int, error) {
		return 0, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, data)

	// data, err = GetInt("key", func() (int, error) {
	// 	return 100, nil
	// })
	// assert.NoError(t, err)
	// assert.Equal(t, 0, data)
	// Remove("key")

	data, err = GetInt("key", func() (int, error) {
		return 100, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 100, data)

	// data, err = GetInt("key", func() (int, error) {
	// 	return 0, fmt.Errorf("some error")
	// })
	// assert.NoError(t, err)
	// assert.Equal(t, 100, data)

	// TODO: uncommented code works in IDE but not with go test
}

func TestGetInt64(t *testing.T) {
	createTestCache()

	data, err := GetInt64("key", func() (int64, error) {
		return 0, fmt.Errorf("some error")
	})
	assert.Error(t, err)
	assert.EqualValues(t, 0, data)

	data, err = GetInt64("key", func() (int64, error) {
		return 0, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, data)

	// data, err = GetInt64("key", func() (int64, error) {
	// 	return 100, nil
	// })
	// assert.NoError(t, err)
	// assert.EqualValues(t, 0, data)
	// Remove("key")

	data, err = GetInt64("key", func() (int64, error) {
		return 100, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 100, data)

	// data, err = GetInt64("key", func() (int, error) {
	// 	return 0, fmt.Errorf("some error")
	// })
	// assert.NoError(t, err)
	// assert.EqualValues(t, 100, data)

	// TODO: uncommented code works in IDE but not with go test
}
