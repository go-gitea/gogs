// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

type testLevel struct {
	Level Level `json:"level"`
}

func TestLevelMarshalUnmarshalJSON(t *testing.T) {
	levelBytes, err := json.Marshal(testLevel{
		Level: INFO,
	})
	assert.NoError(t, err)
	assert.Equal(t, string(makeTestLevelBytes(INFO.String())), string(levelBytes))

	var testLevel testLevel
	err = json.Unmarshal(levelBytes, &testLevel)
	assert.NoError(t, err)
	assert.Equal(t, INFO, testLevel.Level)

	err = json.Unmarshal(makeTestLevelBytes(`FOFOO`), &testLevel)
	assert.NoError(t, err)
	assert.Equal(t, INFO, testLevel.Level)

	err = json.Unmarshal([]byte(fmt.Sprintf(`{"level":%d}`, 2)), &testLevel)
	assert.NoError(t, err)
	assert.Equal(t, INFO, testLevel.Level)

	err = json.Unmarshal([]byte(fmt.Sprintf(`{"level":%d}`, 10012)), &testLevel)
	assert.NoError(t, err)
	assert.Equal(t, INFO, testLevel.Level)

	err = json.Unmarshal([]byte(`{"level":{}}`), &testLevel)
	assert.NoError(t, err)
	assert.Equal(t, INFO, testLevel.Level)

	assert.Equal(t, INFO.String(), Level(1001).String())

	err = json.Unmarshal([]byte(`{"level":{}`), &testLevel.Level)
	assert.Error(t, err)
}

func makeTestLevelBytes(level string) []byte {
	return []byte(fmt.Sprintf(`{"level":"%s"}`, level))
}
