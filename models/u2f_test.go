// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tstranex/u2f"
)

func TestGetU2FRegistrationByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	res, err := GetU2FRegistrationByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "U2F Key", res.Name)

	_, err = GetU2FRegistrationByID(342432)
	assert.Error(t, err)
	assert.True(t, IsErrU2FRegistrationNotExist(err))
}

func TestGetU2FRegistrationsByUID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	res, err := GetU2FRegistrationsByUID(1)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "U2F Key", res[0].Name)
}

func TestU2FRegistration_TableName(t *testing.T) {
	assert.Equal(t, os.Getenv("TABLE_NAME_PREFIX")+"u2f_registration", U2FRegistration{}.TableName())
}

func TestU2FRegistration_UpdateCounter(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	reg := AssertExistsAndLoadBean(t, &U2FRegistration{ID: 1}).(*U2FRegistration)
	reg.Counter = 1
	assert.NoError(t, reg.UpdateCounter())
	AssertExistsIf(t, true, &U2FRegistration{ID: 1, Counter: 1})
}

func TestU2FRegistration_UpdateLargeCounter(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	reg := AssertExistsAndLoadBean(t, &U2FRegistration{ID: 1}).(*U2FRegistration)
	reg.Counter = 0xffffffff
	assert.NoError(t, reg.UpdateCounter())
	AssertExistsIf(t, true, &U2FRegistration{ID: 1, Counter: 0xffffffff})
}

func TestCreateRegistration(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)

	res, err := CreateRegistration(user, "U2F Created Key", &u2f.Registration{Raw: []byte("Test")})
	assert.NoError(t, err)
	assert.Equal(t, "U2F Created Key", res.Name)
	assert.Equal(t, []byte("Test"), res.Raw)

	AssertExistsIf(t, true, &U2FRegistration{Name: "U2F Created Key", UserID: user.ID})
}

func TestDeleteRegistration(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	reg := AssertExistsAndLoadBean(t, &U2FRegistration{ID: 1}).(*U2FRegistration)

	assert.NoError(t, DeleteRegistration(reg))
	AssertNotExistsBean(t, &U2FRegistration{ID: 1})
}
