// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

type makeRequestFunc func(testing.TB, *http.Request, int) *httptest.ResponseRecorder

func TestGPGKeys(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)

	tt := []struct {
		name        string
		makeRequest makeRequestFunc
		token       string
		results     []int
	}{
		{name: "NoLogin", makeRequest: MakeRequest, token: "",
			results: []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
		},
		{name: "LoggedAsUser2", makeRequest: session.MakeRequest, token: token,
			results: []int{http.StatusOK, http.StatusOK, http.StatusNotFound, http.StatusNoContent, http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusCreated, http.StatusNotFound, http.StatusCreated}},
	}

	for _, tc := range tt {
		//Basic test on result code
		t.Run(tc.name, func(t *testing.T) {
			t.Run("ViewOwnGPGKeys", func(t *testing.T) {
				testViewOwnGPGKeys(t, tc.makeRequest, tc.token, tc.results[0])
			})
			t.Run("ViewGPGKeys", func(t *testing.T) {
				testViewGPGKeys(t, tc.makeRequest, tc.token, tc.results[1])
			})
			t.Run("GetGPGKey", func(t *testing.T) {
				testGetGPGKey(t, tc.makeRequest, tc.token, tc.results[2])
			})
			t.Run("DeleteGPGKey", func(t *testing.T) {
				testDeleteGPGKey(t, tc.makeRequest, tc.token, tc.results[3])
			})

			t.Run("CreateInvalidGPGKey", func(t *testing.T) {
				testCreateInvalidGPGKey(t, tc.makeRequest, tc.token, tc.results[4])
			})
			t.Run("CreateNoneRegistredEmailGPGKey", func(t *testing.T) {
				testCreateNoneRegistredEmailGPGKey(t, tc.makeRequest, tc.token, tc.results[5])
			})
			t.Run("CreateValidGPGKey", func(t *testing.T) {
				testCreateValidGPGKey(t, tc.makeRequest, tc.token, tc.results[6])
			})
			t.Run("CreateValidSecondaryEmailGPGKeyNotActivated", func(t *testing.T) {
				testCreateValidSecondaryEmailGPGKey(t, tc.makeRequest, tc.token, tc.results[7])
			})
		})
	}

	//Check state after basic add
	t.Run("CheckState", func(t *testing.T) {

		var keys []*api.GPGKey

		req := NewRequest(t, "GET", "/api/v1/user/gpg_keys?token="+token) //GET all keys
		resp := session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &keys)
		assert.Len(t, keys, 1)

		primaryKey1 := keys[0] //Primary key 1
		assert.EqualValues(t, "38EA3BCED732982C", primaryKey1.KeyID)
		assert.Len(t, primaryKey1.Emails, 1)
		assert.EqualValues(t, "user2@example.com", primaryKey1.Emails[0].Email)
		assert.True(t, primaryKey1.Emails[0].Verified)

		subKey := primaryKey1.SubsKey[0] //Subkey of 38EA3BCED732982C
		assert.EqualValues(t, "70D7C694D17D03AD", subKey.KeyID)
		assert.Empty(t, subKey.Emails)

		var key api.GPGKey
		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(primaryKey1.ID, 10)+"?token="+token) //Primary key 1
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.EqualValues(t, "38EA3BCED732982C", key.KeyID)
		assert.Len(t, key.Emails, 1)
		assert.EqualValues(t, "user2@example.com", key.Emails[0].Email)
		assert.True(t, key.Emails[0].Verified)

		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(subKey.ID, 10)+"?token="+token) //Subkey of 38EA3BCED732982C
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.EqualValues(t, "70D7C694D17D03AD", key.KeyID)
		assert.Empty(t, key.Emails)
	})

	//Check state after basic add
	t.Run("CheckCommits", func(t *testing.T) {
		t.Run("NotSigned", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/not-signed?token="+token)
			resp := session.MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.False(t, branch.Commit.Verification.Verified)
		})

		t.Run("SignedWithNotValidatedEmail", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/good-sign-not-yet-validated?token="+token)
			resp := session.MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.False(t, branch.Commit.Verification.Verified)
		})

		t.Run("SignedWithValidEmail", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/good-sign?token="+token)
			resp := session.MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.True(t, branch.Commit.Verification.Verified)
		})
	})
}

func testViewOwnGPGKeys(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "GET", "/api/v1/user/gpg_keys?token="+token)
	makeRequest(t, req, expected)
}

func testViewGPGKeys(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "GET", "/api/v1/users/user2/gpg_keys?token="+token)
	makeRequest(t, req, expected)
}

func testGetGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "GET", "/api/v1/user/gpg_keys/1?token="+token)
	makeRequest(t, req, expected)
}

func testDeleteGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "DELETE", "/api/v1/user/gpg_keys/1?token="+token)
	makeRequest(t, req, expected)
}

func testCreateGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int, publicKey string) {
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_keys?token="+token, api.CreateGPGKeyOption{
		ArmoredKey: publicKey,
	})
	makeRequest(t, req, expected)
}

func testCreateInvalidGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	testCreateGPGKey(t, makeRequest, token, expected, "invalid_key")
}

func testCreateNoneRegistredEmailGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFmGUygBCACjCNbKvMGgp0fd5vyFW9olE1CLCSyyF9gQN2hSuzmZLuAZF2Kh
dCMCG2T1UwzUB/yWUFWJ2BtCwSjuaRv+cGohqEy6bhEBV90peGA33lHfjx7wP25O
7moAphDOTZtDj1AZfCh/PTcJut8Lc0eRDMhNyp/bYtO7SHNT1Hr6rrCV/xEtSAvR
3b148/tmIBiSadaLwc558KU3ucjnW5RVGins3AjBZ+TuT4XXVH/oeLSeXPSJ5rt1
rHwaseslMqZ4AbvwFLx5qn1OC9rEQv/F548QsA8m0IntLjoPon+6wcubA9Gra21c
Fp6aRYl9x7fiqXDLg8i3s2nKdV7+e6as6Tp9ABEBAAG0FG5vdGtub3duQGV4YW1w
bGUuY29tiQEcBBABAgAGBQJZhlMoAAoJEC8+pvYULDtte/wH/2JNrhmHwDY+hMj0
batIK4HICnkKxjIgbha80P2Ao08NkzSge58fsxiKDFYAQjHui+ZAw4dq79Ax9AOO
Iv2GS9+DUfWhrb6RF+vNuJldFzcI0rTW/z2q+XGKrUCwN3khJY5XngHfQQrdBtMK
qsoUXz/5B8g422RTbo/SdPsyYAV6HeLLeV3rdgjI1fpaW0seZKHeTXQb/HvNeuPg
qz+XV1g6Gdqa1RjDOaX7A8elVKxrYq3LBtc93FW+grBde8n7JL0zPM3DY+vJ0IJZ
INx/MmBfmtCq05FqNclvU+sj2R3N1JJOtBOjZrJHQbJhzoILou8AkxeX1A+q9OAz
1geiY5E=
=TkP3
-----END PGP PUBLIC KEY BLOCK-----`)
}

func testCreateValidGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	//User2 <user2@example.com> //primary & activated
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFmGVsMBCACuxgZ7W7rI9xN08Y4M7B8yx/6/I4Slm94+wXf8YNRvAyqj30dW
VJhyBcnfNRDLKSQp5o/hhfDkCgdqBjLa1PnHlGS3PXJc0hP/FyYPD2BFvNMPpCYS
eu3T1qKSNXm6X0XOWD2LIrdiDC8HaI9FqZVMI/srMK2CF8XCL2m67W1FuoPlWzod
5ORy0IZB7spoF0xihmcgnEGElRmdo5w/vkGH8U7Zyn9Eb57UVFeafgeskf4wqB23
BjbMdW2YaB+yzMRwYgOnD5lnBD4uqSmvjaV9C0kxn7x+oJkkiRV8/z1cNcO+BaeQ
Akh/yTTeTzYGSc/ZOqCX1O+NOPgSeixVlqenABEBAAG0GVVzZXIyIDx1c2VyMkBl
eGFtcGxlLmNvbT6JAVQEEwEIAD4WIQRXgbSh0TtGbgRd7XI46jvO1zKYLAUCWYZW
wwIbAwUJA8JnAAULCQgHAgYVCAkKCwIEFgIDAQIeAQIXgAAKCRA46jvO1zKYLF/e
B/91wm2KLMIQBZBA9WA2/+9rQWTo9EqgYrXN60rEzX3cYJWXZiE4DrKR1oWDGNLi
KXOCW62snvJldolBqq0ZqaKvPKzl0Y5TRqbYEc9AjUSqgRin1b+G2DevLGT4ibq+
7ocQvz0XkASEUAgHahp0Ubiiib1521WwT/duL+AG8Gg0+DK09RfV3eX5/EOkQCKv
8cutqgsd2Smz40A8wXuJkRcipZBtrB/GkUaZ/eJdwEeSYZjEA9GWF61LJT2stvRN
HCk7C3z3pVEek1PluiFs/4VN8BG8yDzW4c0tLty4Fj3VwPqwIbB5AJbquVfhQCb4
Eep2lm3Lc9b1OwO5N3coPJkouQENBFmGVsMBCADAGba2L6NCOE1i3WIP6CPzbdOo
N3gdTfTgccAx9fNeon9jor+3tgEjlo9/6cXiRoksOV6W4wFab/ZwWgwN6JO4CGvZ
Wi7EQwMMMp1E36YTojKQJrcA9UvMnTHulqQQ88F5E845DhzFQM3erv42QZZMBAX3
kXCgy1GNFocl6tLUvJdEqs+VcJGGANMpmzE4WLa8KhSYnxipwuQ62JBy9R+cHyKT
OARk8znRqSu5bT3LtlrZ/HXu+6Oy4+2uCdNzZIh5J5tPS7CPA6ptl88iGVBte/CJ
7cjgJWSQqeYp2Y5QvsWAivkQ4Ww9plHbbwV0A2eaHsjjWzlUl3HoJ/snMOhBABEB
AAGJATwEGAEIACYWIQRXgbSh0TtGbgRd7XI46jvO1zKYLAUCWYZWwwIbDAUJA8Jn
AAAKCRA46jvO1zKYLBwLCACQOpeRVrwIKVaWcPMYjVHHJsGscaLKpgpARAUgbiG6
Cbc2WI8Sm3fRwrY0VAfN+u9QwrtvxANcyB3vTgTzw7FimfhOimxiTSO8HQCfjDZF
Xly8rq+Fua7+ClWUpy21IekW41VvZYjH2sL6EVP+UcEOaGAyN53XfhaRVZPhNtZN
NKAE9N5EG3rbsZ33LzJj40rEKlzFSseAAPft8qA3IXjzFBx+PQXHMpNCagL79he6
lqockTJ+oPmta4CF/J0U5LUr1tOZXheL3TP6m8d08gDrtn0YuGOPk87i9sJz+jR9
uy6MA3VSB99SK9ducGmE1Jv8mcziREroz2TEGr0zPs6h
=J59D
-----END PGP PUBLIC KEY BLOCK-----`)
}

func testCreateValidSecondaryEmailGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	//User2 <user2-2@example.com> //secondary and not activated
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
6qgtsUdi5eviONRkBgeZtN3oxA==
=MgDv
-----END PGP PUBLIC KEY BLOCK-----`)
}
