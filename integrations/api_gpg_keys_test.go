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
		{
			name: "NoLogin", makeRequest: MakeRequest, token: "",
			results: []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
		},
		{
			name: "LoggedAsUser2", makeRequest: session.MakeRequest, token: token,
			results: []int{http.StatusOK, http.StatusOK, http.StatusNotFound, http.StatusNoContent, http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusCreated, http.StatusCreated},
		},
	}

	for _, tc := range tt {
		// Basic test on result code
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
			t.Run("CreateValidSecondaryEmailGPGKey", func(t *testing.T) {
				testCreateValidSecondaryEmailGPGKey(t, tc.makeRequest, tc.token, tc.results[7])
			})
		})
	}

	// Check state after basic add
	t.Run("CheckState", func(t *testing.T) {
		var keys []*api.GPGKey

		req := NewRequest(t, "GET", "/api/v1/user/gpg_keys?token="+token) // GET all keys
		resp := session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &keys)

		primaryKey1 := keys[0] // Primary key 1
		assert.EqualValues(t, "38EA3BCED732982C", primaryKey1.KeyID)
		assert.EqualValues(t, 1, len(primaryKey1.Emails))
		assert.EqualValues(t, "user2@example.com", primaryKey1.Emails[0].Email)
		assert.EqualValues(t, true, primaryKey1.Emails[0].Verified)

		subKey := primaryKey1.SubsKey[0] // Subkey of 38EA3BCED732982C
		assert.EqualValues(t, "70D7C694D17D03AD", subKey.KeyID)
		assert.EqualValues(t, 0, len(subKey.Emails))

		primaryKey2 := keys[1] // Primary key 2
		assert.EqualValues(t, "FABF39739FE1E927", primaryKey2.KeyID)
		assert.EqualValues(t, 1, len(primaryKey2.Emails))
		assert.EqualValues(t, "user21@example.com", primaryKey2.Emails[0].Email)
		assert.EqualValues(t, false, primaryKey2.Emails[0].Verified)

		var key api.GPGKey
		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(primaryKey1.ID, 10)+"?token="+token) // Primary key 1
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.EqualValues(t, "38EA3BCED732982C", key.KeyID)
		assert.EqualValues(t, 1, len(key.Emails))
		assert.EqualValues(t, "user2@example.com", key.Emails[0].Email)
		assert.EqualValues(t, true, key.Emails[0].Verified)

		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(subKey.ID, 10)+"?token="+token) // Subkey of 38EA3BCED732982C
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.EqualValues(t, "70D7C694D17D03AD", key.KeyID)
		assert.EqualValues(t, 0, len(key.Emails))

		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(primaryKey2.ID, 10)+"?token="+token) // Primary key 2
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.EqualValues(t, "FABF39739FE1E927", key.KeyID)
		assert.EqualValues(t, 1, len(key.Emails))
		assert.EqualValues(t, "user21@example.com", key.Emails[0].Email)
		assert.EqualValues(t, false, key.Emails[0].Verified)
	})

	// Check state after basic add
	t.Run("CheckCommits", func(t *testing.T) {
		t.Run("NotSigned", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/not-signed?token="+token)
			resp := session.MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.EqualValues(t, false, branch.Commit.Verification.Verified)
		})

		t.Run("SignedWithNotValidatedEmail", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/good-sign-not-yet-validated?token="+token)
			resp := session.MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.EqualValues(t, false, branch.Commit.Verification.Verified)
		})

		t.Run("SignedWithValidEmail", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/good-sign?token="+token)
			resp := session.MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.EqualValues(t, true, branch.Commit.Verification.Verified)
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
	// User2 <user2@example.com> //primary & activated
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
	// User2 <user21@example.com> //secondary and not activated
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFmGWN4BCAC18V4tVGO65VLCV7p14FuXJlUtZ5CuYMvgEkcOqrvRaBSW9ao4
PGESOhJpfWpnW3QgJniYndLzPpsmdHEclEER6aZjiNgReWPOjHD5tykWocZAJqXD
eY1ym59gvVMLcfbV2yQsyR2hbJlc+dJsl16tigSEe3nwxZSw2IsW92pgEzT9JNUr
Q+mC8dw4dqY0tYmFazYUGNxufUc/twgQT/Or1aNs0az5Q6Jft4rrTRsh/S7We0VB
COKGkdcQyYgAls7HJBuPjQRi6DM9VhgBSHLAgSLyaUcZvhZBJr8Qe/q4PP3/kYDJ
wm4RMnjOLz2pFZPgtRqgcAwpmFtLrACbEB3JABEBAAG0GlVzZXIyIDx1c2VyMjFA
ZXhhbXBsZS5jb20+iQFUBBMBCAA+FiEEPOLHOjPSO42DWM57+r85c5/h6ScFAlmG
WN4CGwMFCQPCZwAFCwkIBwIGFQgJCgsCBBYCAwECHgECF4AACgkQ+r85c5/h6Sfx
Lgf/dq64NBV8+X9an3seaLxePRviva48e4K67/wV/JxtXNO5Z/DhMGz5kHXCsG9D
CXuWYO8ehlTjEnMZ6qqdDnY+H6bQsb2OS5oPn4RwpPXslAjEKtojPAr0dDsMS2DB
dUuIm1AoOnewOVO0OFRf1EqX1bivxnN0FVMcO0m8AczfnKDaGb0y/qg/Y9JAsKqp
j5pZNMWUkntRtGySeJ4CVJMmkVKJAHsa1Qj6MKdFeid4h4y94cBJ4ZdyBxNdpQOx
ydf0doicovfeqGNO4oWzsGP4RBK2CqGPCUT+EFl20jPvMkKwOjxgqc8p0z3b2UT9
+9bnmCGHgF/fW1HJ3iKmfFPqnLkBDQRZhljeAQgA5AirU/NJGgm19ZJYFOiHftjS
azbrPxGeD3cSqmvDPIMc1DNZGfQV5D4EVumnVbQBtL6xHFoGKz9KisUMbe4a/X2J
S8JmIphQWG0vMJX1DaZIzr2gT71MnPD7JMGsSUCh5dIKpTNTZX4w+oGPGOu0/UlL
x0448AryKwp30J2p6D4GeI0nb03n35S2lTOpnHDn1wj7Jl/8LS2fdFOdNaNHXSZe
twdSwJKhyBEiScgeHBDyKqo8zWkYoSb9eA2HiYlbVaiNtp24KP1mIEpiUdrRjWno
zauYSZGHZlOFMgF4dKWuetPiuH9m7UYZGKyMLfQ9vYFb+xcPh2bLCQHJ1OEmMQAR
AQABiQE8BBgBCAAmFiEEPOLHOjPSO42DWM57+r85c5/h6ScFAlmGWN4CGwwFCQPC
ZwAACgkQ+r85c5/h6Sfjfwf+O4WEjRdvPJLxNy7mfAGoAqDMHIwyH/tVzYgyVhnG
h/+cfRxJbGc3rpjYdr8dmvghzjEAout8uibPWaIqs63RCAPGPqgWLfxNO5c8+y8V
LZMVOTV26l2olkkdBWAuhLqKTNh6TiQva03yhOgHWj4XDvFfxICWPFXVd6t5ELpD
iApGu1OAj8JfhmzbG03Yzx+Ku7bWDxMonx3V/IDEu5LS5zrboHYDKCA53bXXghoi
Aceqql+PKrDwEjoY4bptwMHLmcjGjdCQ//Qx1neho7nZcS7xjTucY8gQuulwCyXF
y6wM+wMz8dunIG9gw4+Re6c4Rz9tX1kzxLrU7Pl21tMqfg==
=0N/9
-----END PGP PUBLIC KEY BLOCK-----`)
}
