// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestCheckArmoredGPGKeyString(t *testing.T) {
	testGPGArmor := `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFh91QoBCADciaDd7aqegYkn4ZIG7J0p1CRwpqMGjxFroJEMg6M1ZiuEVTRv
z49P4kcr1+98NvFmcNc+x5uJgvPCwr/N8ZW5nqBUs2yrklbFF4MeQomyZJJegP8m
/dsRT3BwIT8YMUtJuCj0iqD9vuKYfjrztcMgC1sYwcE9E9OlA0pWBvUdU2i0TIB1
vOq6slWGvHHa5l5gPfm09idlVxfH5+I+L1uIMx5ovbiVVU5x2f1AR1T18f0t2TVN
0agFTyuoYE1ATmvJHmMcsfgM1Gpd9hIlr9vlupT2kKTPoNzVzsJsOU6Ku/Lf/bac
mF+TfSbRCtmG7dkYZ4metLj7zG/WkW8IvJARABEBAAG0HUFudG9pbmUgR0lSQVJE
IDxzYXBrQHNhcGsuZnI+iQFUBBMBCAA+FiEEEIOwJg/1vpF1itJ4roJVuKDYKOQF
Alh91QoCGwMFCQPCZwAFCwkIBwIGFQgJCgsCBBYCAwECHgECF4AACgkQroJVuKDY
KORreggAlIkC2QjHP5tb7b0+LksB2JMXdY+UzZBcJxtNmvA7gNQaGvWRrhrbePpa
MKDP+3A4BPDBsWFbbB7N56vQ5tROpmWbNKuFOVER4S1bj0JZV0E+xkDLqt9QwQtQ
ojd7oIZJwDUwdud1PvCza2mjgBqqiFE+twbc3i9xjciCGspMniUul1eQYLxRJ0w+
sbvSOUnujnq5ByMSz9ij00O6aiPfNQS5oB5AALfpjYZDvWAAljLVrtmlQJWZ6dZo
T/YNwsW2dECPuti8+Nmu5FxPGDTXxdbnRaeJTQ3T6q1oUVAv7yTXBx5NXfXkMa5i
iEayQIH8Joq5Ev5ja/lRGQQhArMQ2bkBDQRYfdUKAQgAv7B3coLSrOQbuTZSlgWE
QeT+7DWbmqE1LAQA1pQPcUPXLBUVd60amZJxF9nzUYcY83ylDi0gUNJS+DJGOXpT
pzX2IOuOMGbtUSeKwg5s9O4SUO7f2yCc3RGaegER5zgESxelmOXG+b/hoNt7JbdU
JtxcnLr91Jw2PBO/Xf0ZKJ01CQG2Yzdrrj6jnrHyx94seHy0i6xH1o0OuvfVMLfN
/Vbb/ZHh6ym2wHNqRX62b0VAbchcJXX/MEehXGknKTkO6dDUd+mhRgWMf9ZGRFWx
ag4qALimkf1FXtAyD0vxFYeyoWUQzrOvUsm2BxIN/986R08fhkBQnp5nz07mrU02
cQARAQABiQE8BBgBCAAmFiEEEIOwJg/1vpF1itJ4roJVuKDYKOQFAlh91QoCGwwF
CQPCZwAACgkQroJVuKDYKOT32wf/UZqMdPn5OhyhffFzjQx7wolrf92WkF2JkxtH
6c3Htjlt/p5RhtKEeErSrNAxB4pqB7dznHaJXiOdWEZtRVXXjlNHjrokGTesqtKk
lHWtK62/MuyLdr+FdCl68F3ewuT2iu/MDv+D4HPqA47zma9xVgZ9ZNwJOpv3fCOo
RfY66UjGEnfgYifgtI5S84/mp2jaSc9UNvlZB6RSf8cfbJUL74kS2lq+xzSlf0yP
Av844q/BfRuVsJsK1NDNG09LC30B0l3LKBqlrRmRTUMHtgchdX2dY+p7GPOoSzlR
MkM/fdpyc2hY7Dl/+qFmN5MG5yGmMpQcX+RNNR222ibNC1D3wg==
=i9b7
-----END PGP PUBLIC KEY BLOCK-----`

	key, err := checkArmoredGPGKeyString(testGPGArmor)
	assert.NoError(t, err, "Could not parse a valid GPG public armored rsa key", key)
	// TODO verify value of key
}

func TestCheckArmoredbrainpoolP256r1GPGKeyString(t *testing.T) {
	testGPGArmor := `-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2

mFMEV6HwkhMJKyQDAwIIAQEHAgMEUsvJO/j5dFMRRj67qeZC9fSKBsGZdOHRj2+6
8wssmbUuLTfT/ZjIbExETyY8hFnURRGpD2Ifyz0cKjXcbXfJtrQTRm9vYmFyIDxm
b29AYmFyLmRlPoh/BBMTCAAnBQJZOsDIAhsDBQkJZgGABQsJCAcCBhUICQoLAgQW
AgMBAh4BAheAAAoJEGuJTd/DBMzmNVQA/2beUrv1yU4gyvCiPDEm3pK42cSfaL5D
muCtPCUg9hlWAP4yq6M78NW8STfsXgn6oeziMYiHSTmV14nOamLuwwDWM7hXBFeh
8JISCSskAwMCCAEBBwIDBG3A+XfINAZp1CTse2mRNgeUE5DbUtEpO8ALXKA1UQsQ
DLKq27b7zTgawgXIGUGP6mWsJ5oH7MNAJ/uKTsYmX40DAQgHiGcEGBMIAA8FAleh
8JICGwwFCQlmAYAACgkQa4lN38MEzOZwKAD/QKyerAgcvzzLaqvtap3XvpYcw9tc
OyjLLnFQiVmq7kEA/0z0CQe3ZQiQIq5zrs7Nh1XRkFAo8GlU/SGC9XFFi722
=ZiSe
-----END PGP PUBLIC KEY BLOCK-----`

	key, err := checkArmoredGPGKeyString(testGPGArmor)
	assert.NoError(t, err, "Could not parse a valid GPG public armored brainpoolP256r1 key", key)
	// TODO verify value of key
}

func TestExtractSignature(t *testing.T) {
	testGPGArmor := `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFh91QoBCADciaDd7aqegYkn4ZIG7J0p1CRwpqMGjxFroJEMg6M1ZiuEVTRv
z49P4kcr1+98NvFmcNc+x5uJgvPCwr/N8ZW5nqBUs2yrklbFF4MeQomyZJJegP8m
/dsRT3BwIT8YMUtJuCj0iqD9vuKYfjrztcMgC1sYwcE9E9OlA0pWBvUdU2i0TIB1
vOq6slWGvHHa5l5gPfm09idlVxfH5+I+L1uIMx5ovbiVVU5x2f1AR1T18f0t2TVN
0agFTyuoYE1ATmvJHmMcsfgM1Gpd9hIlr9vlupT2kKTPoNzVzsJsOU6Ku/Lf/bac
mF+TfSbRCtmG7dkYZ4metLj7zG/WkW8IvJARABEBAAG0HUFudG9pbmUgR0lSQVJE
IDxzYXBrQHNhcGsuZnI+iQFUBBMBCAA+FiEEEIOwJg/1vpF1itJ4roJVuKDYKOQF
Alh91QoCGwMFCQPCZwAFCwkIBwIGFQgJCgsCBBYCAwECHgECF4AACgkQroJVuKDY
KORreggAlIkC2QjHP5tb7b0+LksB2JMXdY+UzZBcJxtNmvA7gNQaGvWRrhrbePpa
MKDP+3A4BPDBsWFbbB7N56vQ5tROpmWbNKuFOVER4S1bj0JZV0E+xkDLqt9QwQtQ
ojd7oIZJwDUwdud1PvCza2mjgBqqiFE+twbc3i9xjciCGspMniUul1eQYLxRJ0w+
sbvSOUnujnq5ByMSz9ij00O6aiPfNQS5oB5AALfpjYZDvWAAljLVrtmlQJWZ6dZo
T/YNwsW2dECPuti8+Nmu5FxPGDTXxdbnRaeJTQ3T6q1oUVAv7yTXBx5NXfXkMa5i
iEayQIH8Joq5Ev5ja/lRGQQhArMQ2bkBDQRYfdUKAQgAv7B3coLSrOQbuTZSlgWE
QeT+7DWbmqE1LAQA1pQPcUPXLBUVd60amZJxF9nzUYcY83ylDi0gUNJS+DJGOXpT
pzX2IOuOMGbtUSeKwg5s9O4SUO7f2yCc3RGaegER5zgESxelmOXG+b/hoNt7JbdU
JtxcnLr91Jw2PBO/Xf0ZKJ01CQG2Yzdrrj6jnrHyx94seHy0i6xH1o0OuvfVMLfN
/Vbb/ZHh6ym2wHNqRX62b0VAbchcJXX/MEehXGknKTkO6dDUd+mhRgWMf9ZGRFWx
ag4qALimkf1FXtAyD0vxFYeyoWUQzrOvUsm2BxIN/986R08fhkBQnp5nz07mrU02
cQARAQABiQE8BBgBCAAmFiEEEIOwJg/1vpF1itJ4roJVuKDYKOQFAlh91QoCGwwF
CQPCZwAACgkQroJVuKDYKOT32wf/UZqMdPn5OhyhffFzjQx7wolrf92WkF2JkxtH
6c3Htjlt/p5RhtKEeErSrNAxB4pqB7dznHaJXiOdWEZtRVXXjlNHjrokGTesqtKk
lHWtK62/MuyLdr+FdCl68F3ewuT2iu/MDv+D4HPqA47zma9xVgZ9ZNwJOpv3fCOo
RfY66UjGEnfgYifgtI5S84/mp2jaSc9UNvlZB6RSf8cfbJUL74kS2lq+xzSlf0yP
Av844q/BfRuVsJsK1NDNG09LC30B0l3LKBqlrRmRTUMHtgchdX2dY+p7GPOoSzlR
MkM/fdpyc2hY7Dl/+qFmN5MG5yGmMpQcX+RNNR222ibNC1D3wg==
=i9b7
-----END PGP PUBLIC KEY BLOCK-----`
	keys, err := checkArmoredGPGKeyString(testGPGArmor)
	if !assert.NotEmpty(t, keys) {
		return
	}
	ekey := keys[0]
	assert.NoError(t, err, "Could not parse a valid GPG armored key", ekey)

	pubkey := ekey.PrimaryKey
	content, err := base64EncPubKey(pubkey)
	assert.NoError(t, err, "Could not base64 encode a valid PublicKey content", ekey)

	key := &GPGKey{
		KeyID:             pubkey.KeyIdString(),
		Content:           content,
		CreatedUnix:       timeutil.TimeStamp(pubkey.CreationTime.Unix()),
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}

	cannotsignkey := &GPGKey{
		KeyID:             pubkey.KeyIdString(),
		Content:           content,
		CreatedUnix:       timeutil.TimeStamp(pubkey.CreationTime.Unix()),
		CanSign:           false,
		CanEncryptComms:   false,
		CanEncryptStorage: false,
		CanCertify:        false,
	}

	testGoodSigArmor := `-----BEGIN PGP SIGNATURE-----

iQEzBAABCAAdFiEEEIOwJg/1vpF1itJ4roJVuKDYKOQFAljAiQIACgkQroJVuKDY
KORvCgf6A/Ehh0r7QbO2tFEghT+/Ab+bN7jRN3zP9ed6/q/ophYmkrU0NibtbJH9
AwFVdHxCmj78SdiRjaTKyevklXw34nvMftmvnOI4lBNUdw6KWl25/n/7wN0l2oZW
rW3UawYpZgodXiLTYarfEimkDQmT67ArScjRA6lLbkEYKO0VdwDu+Z6yBUH3GWtm
45RkXpnsF6AXUfuD7YxnfyyDE1A7g7zj4vVYUAfWukJjqow/LsCUgETETJOqj9q3
52/oQDs04fVkIEtCDulcY+K/fKlukBPJf9WceNDEqiENUzN/Z1y0E+tJ07cSy4bk
yIJb+d0OAaG8bxloO7nJq4Res1Qa8Q==
=puvG
-----END PGP SIGNATURE-----`
	testGoodPayload := `tree 56ae8d2799882b20381fc11659db06c16c68c61a
parent c7870c39e4e6b247235ca005797703ec4254613f
author Antoine GIRARD <sapk@sapk.fr> 1489012989 +0100
committer Antoine GIRARD <sapk@sapk.fr> 1489012989 +0100

Goog GPG
`

	testBadSigArmor := `-----BEGIN PGP SIGNATURE-----

iQEzBAABCAAdFiEE5yr4rn9ulbdMxJFiPYI/ySNrtNkFAljAiYkACgkQPYI/ySNr
tNmDdQf+NXhVRiOGt0GucpjJCGrOnK/qqVUmQyRUfrqzVUdb/1/Ws84V5/wE547I
6z3oxeBKFsJa1CtIlxYaUyVhYnDzQtphJzub+Aw3UG0E2ywiE+N7RCa1Ufl7pPxJ
U0SD6gvNaeTDQV/Wctu8v8DkCtEd3N8cMCDWhvy/FQEDztVtzm8hMe0Vdm0ozEH6
P0W93sDNkLC5/qpWDN44sFlYDstW5VhMrnF0r/ohfaK2kpYHhkPk7WtOoHSUwQSg
c4gfhjvXIQrWFnII1Kr5jFGlmgNSR02qpb31VGkMzSnBhWVf2OaHS/kI49QHJakq
AhVDEnoYLCgoDGg9c3p1Ll2452/c6Q==
=uoGV
-----END PGP SIGNATURE-----`
	testBadPayload := `tree 3074ff04951956a974e8b02d57733b0766f7cf6c
parent fd3577542f7ad1554c7c7c0eb86bb57a1324ad91
author Antoine GIRARD <sapk@sapk.fr> 1489013107 +0100
committer Antoine GIRARD <sapk@sapk.fr> 1489013107 +0100

Unknown GPG key with good email
`
	// Reading Sign
	goodSig, err := extractSignature(testGoodSigArmor)
	assert.NoError(t, err, "Could not parse a valid GPG armored signature", testGoodSigArmor)
	badSig, err := extractSignature(testBadSigArmor)
	assert.NoError(t, err, "Could not parse a valid GPG armored signature", testBadSigArmor)

	// Generating hash of commit
	goodHash, err := populateHash(goodSig.Hash, []byte(testGoodPayload))
	assert.NoError(t, err, "Could not generate a valid hash of payload", testGoodPayload)
	badHash, err := populateHash(badSig.Hash, []byte(testBadPayload))
	assert.NoError(t, err, "Could not generate a valid hash of payload", testBadPayload)

	// Verify
	err = verifySign(goodSig, goodHash, key)
	assert.NoError(t, err, "Could not validate a good signature")
	err = verifySign(badSig, badHash, key)
	assert.Error(t, err, "Validate a bad signature")
	err = verifySign(goodSig, goodHash, cannotsignkey)
	assert.Error(t, err, "Validate a bad signature with a kay that can not sign")
}

func TestCheckGPGUserEmail(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	_ = AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)

	testEmailWithUpperCaseLetters := `-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v1

mQENBFlEBvMBCADe+EQcfv/aKbMFy7YB8e/DE+hY39sfjvdvSgeXtNhfmYvIOUjT
ORMCvce2Oxzb3HTI0rjYsJpzo9jEQ53dB3vdr0ne5Juby6N7QPjof3NR+ko50Ki2
0ilOjYuA0v6VHLIn70UBa9NEf+XDuE7P+Lbtl2L9B9OMXtcTAZoA3cJySgtNFNIG
AVefPi8LeOcekL39wxJEA8OzdCyO5oENEwAG1tzjy9DDNJf74/dBBh2NiXeSeMxZ
RYeYzqEa2UTDP1fkUl7d2/hV36cKZWZr+l4SQ5bM7HeLj2SsfabLfqKoVWgkfAzQ
VwtkbRpzMiDLMte2ZAyTJUc+77YbFoyAmOcjABEBAAG0HFVzZXIgT25lIDxVc2Vy
MUBFeGFtcGxlLmNvbT6JATgEEwECACIFAllEBvMCGwMGCwkIBwMCBhUIAgkKCwQW
AgMBAh4BAheAAAoJEFMOzOY274DFw5EIAKc4jiYaMb1HDKrSv0tphgNxPFEY83/J
9CZggO7BINxlb7z/lH1i0U2h2Ha9E3VJTJQF80zBCaIvtU2UNrgVmSKoc0BdE/2S
rS9MAl29sXxf1BfvXHu12Suvo8O/ZFP45Vm/3kkHuasHyOV1GwUWnynt1qo0zUEn
WMIcB8USlmMT1TnSb10YKBd/BpGF3crFDJLfAHRumZUk4knDDWUOWy5RCOG8cedc
VTAhfdoKRRO3PchOfz6Rls/hew12mRNayqxuLQl2+BX+BWu+25dR3qyiS+twLbk6
Rjpb0S+RQTkYIUoI0SEZpxcTZso11xF5KNpKZ9aAoiLJqkNF5h4oPSe5AQ0EWUQG
8wEIALiMMqh3NF3ON/z7hQfeU24bCl/WdfJwCR9CWU/jx4X4gZq2C2aGtytGN5g/
qoYQ3poTOPzh/4Dvs+r6CtHqi0CvPiEOfSxzmaK+F+vA0GMn2i3Sx5gq/VB0mr+j
RIYMCjf68Tifo2RAT0VDzn6t304l5+VPr4OgbobMRH+wDe7Hhd2pZXl7ty8DooBn
vqaqoKgdiccUXGBKe4Oihl/oZ4qrYH6K4ACP1Sco1rs4mNeKDAW8k/Y7zLjg6d59
g0YQ1YI+CX/bKB7/cpMHLupyMLqvCcqIpjBXRJNMdjuMHgKckjr89DwnqXqgXz7W
u0B39MZQn9nn6vq8BdkoDFgrTQ8AEQEAAYkBHwQYAQIACQUCWUQG8wIbDAAKCRBT
DszmNu+Axf4IB/0S9NTc6kpwW+ZPZQNTWR5oKDEaXVCRLccOlkt33txMvk/z2jNM
trEke99ss5L1bRyWB5fRA+XVsPmW9kIk8pmGFmxqp2nSxr9m9rlL5oTYH8u6dfSm
zwGhqkfITjPI7hyNN52PLANwoS0o4dLzIE65ewigx6cnRlrT2IENObxG/tlxaYg1
NHahJX0uFlVk0W0bLBrs3fTDw1lS/N8HpyQb+5ryQmiIb2a48aygCS/h2qeRlX1d
Q0KHb+QcycSgbDx0ZAvdIacuKvBBcbxrsmFUI4LR+oIup0G9gUc0roPvr014jYQL
7f8r/8fpcN8t+I/41QHCs6L/BEIdTHW3rTQ6
=zHo9
-----END PGP PUBLIC KEY BLOCK-----`

	keys, err := AddGPGKey(1, testEmailWithUpperCaseLetters, "", "")
	assert.NoError(t, err)
	if assert.NotEmpty(t, keys) {
		key := keys[0]
		if assert.Len(t, key.Emails, 1) {
			assert.Equal(t, "user1@example.com", key.Emails[0].Email)
		}
	}
}

func TestCheckGParseGPGExpire(t *testing.T) {
	testIssue6599 := `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
epiDVQ==
=VSKJ
-----END PGP PUBLIC KEY BLOCK-----
`
	keys, err := checkArmoredGPGKeyString(testIssue6599)
	assert.NoError(t, err)
	if assert.NotEmpty(t, keys) {
		ekey := keys[0]
		expire := getExpiryTime(ekey)
		assert.Equal(t, time.Unix(1586105389, 0), expire)
	}
}
