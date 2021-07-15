// Copyright 2017 Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// GPGKey a user GPG key to sign commit and tag in repository
type GPGKey struct {
	ID                int64          `json:"id"`
	PrimaryKeyID      string         `json:"primary_key_id"`
	KeyID             string         `json:"key_id"`
	PublicKey         string         `json:"public_key"`
	Emails            []*GPGKeyEmail `json:"emails"`
	SubsKey           []*GPGKey      `json:"subkeys"`
	CanSign           bool           `json:"can_sign"`
	CanEncryptComms   bool           `json:"can_encrypt_comms"`
	CanEncryptStorage bool           `json:"can_encrypt_storage"`
	CanCertify        bool           `json:"can_certify"`
	Verified          bool           `json:"verified"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at,omitempty"`
	// swagger:strfmt date-time
	Expires time.Time `json:"expires_at,omitempty"`
}

// GPGKeyEmail an email attached to a GPGKey
// swagger:model GPGKeyEmail
type GPGKeyEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// CreateGPGKeyOption options create user GPG key
type CreateGPGKeyOption struct {
	// An armored GPG key to add
	//
	// required: true
	// unique: true
	ArmoredKey string `json:"armored_public_key" binding:"Required"`
	Signature  string `json:"armored_signature,omitempty"`
}

// VerifyGPGKeyOption options verifies user GPG key
type VerifyGPGKeyOption struct {
	// An Signature for a GPG key token
	//
	// required: true
	KeyID     string `json:"key_id" binding:"Required"`
	Signature string `json:"armored_signature" binding:"Required"`
}
