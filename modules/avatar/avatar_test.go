// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package avatar

import (
	"io/ioutil"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func Test_RandomImage(t *testing.T) {
	_, err := RandomImage([]byte("gogs@local"))
	assert.NoError(t, err)

	_, err = RandomImageSize(0, []byte("gogs@local"))
	assert.Error(t, err)
}

func Test_PrepareWithPNG(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	data, err := ioutil.ReadFile("testdata/avatar.png")
	assert.NoError(t, err)

	imgPtr, err := Prepare(data)
	assert.NoError(t, err)

	assert.Equal(t, 290, (*imgPtr).Bounds().Max.X)
	assert.Equal(t, 290, (*imgPtr).Bounds().Max.Y)
}

func Test_PrepareWithJPEG(t *testing.T) {
	setting.Avatar.MaxWidth = 4096
	setting.Avatar.MaxHeight = 4096

	data, err := ioutil.ReadFile("testdata/avatar.jpeg")
	assert.NoError(t, err)

	imgPtr, err := Prepare(data)
	assert.NoError(t, err)

	assert.Equal(t, 290, (*imgPtr).Bounds().Max.X)
	assert.Equal(t, 290, (*imgPtr).Bounds().Max.Y)
}

func Test_PrepareWithInvalidImage(t *testing.T) {
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5

	_, err := Prepare([]byte{})
	assert.EqualError(t, err, "DecodeConfig: image: unknown format")
}

func Test_PrepareWithInvalidImageSize(t *testing.T) {
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5

	data, err := ioutil.ReadFile("testdata/avatar.png")
	assert.NoError(t, err)

	_, err = Prepare(data)
	assert.EqualError(t, err, "Image width is too large: 10 > 5")
}
