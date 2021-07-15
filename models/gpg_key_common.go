// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"strings"
	"time"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/keybase/go-crypto/openpgp/packet"
)

//   __________________  ________   ____  __.
//  /  _____/\______   \/  _____/  |    |/ _|____ ___.__.
// /   \  ___ |     ___/   \  ___  |      <_/ __ <   |  |
// \    \_\  \|    |   \    \_\  \ |    |  \  ___/\___  |
//  \______  /|____|    \______  / |____|__ \___  > ____|
//         \/                  \/          \/   \/\/
// _________
// \_   ___ \  ____   _____   _____   ____   ____
// /    \  \/ /  _ \ /     \ /     \ /  _ \ /    \
// \     \___(  <_> )  Y Y  \  Y Y  (  <_> )   |  \
//  \______  /\____/|__|_|  /__|_|  /\____/|___|  /
//         \/             \/      \/            \/

// This file provides common functions relating to GPG Keys

// checkArmoredGPGKeyString checks if the given key string is a valid GPG armored key.
// The function returns the actual public key on success
func checkArmoredGPGKeyString(content string) (openpgp.EntityList, error) {
	list, err := openpgp.ReadArmoredKeyRing(strings.NewReader(content))
	if err != nil {
		return nil, ErrGPGKeyParsing{err}
	}
	return list, nil
}

// base64EncPubKey encode public key content to base 64
func base64EncPubKey(pubkey *packet.PublicKey) (string, error) {
	var w bytes.Buffer
	err := pubkey.Serialize(&w)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(w.Bytes()), nil
}

func readerFromBase64(s string) (io.Reader, error) {
	bs, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(bs), nil
}

// base64DecPubKey decode public key content from base 64
func base64DecPubKey(content string) (*packet.PublicKey, error) {
	b, err := readerFromBase64(content)
	if err != nil {
		return nil, err
	}
	// Read key
	p, err := packet.Read(b)
	if err != nil {
		return nil, err
	}
	// Check type
	pkey, ok := p.(*packet.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not a public key")
	}
	return pkey, nil
}

// getExpiryTime extract the expire time of primary key based on sig
func getExpiryTime(e *openpgp.Entity) time.Time {
	expiry := time.Time{}
	// Extract self-sign for expire date based on : https://github.com/golang/crypto/blob/master/openpgp/keys.go#L165
	var selfSig *packet.Signature
	for _, ident := range e.Identities {
		if selfSig == nil {
			selfSig = ident.SelfSignature
		} else if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
			selfSig = ident.SelfSignature
			break
		}
	}
	if selfSig.KeyLifetimeSecs != nil {
		expiry = e.PrimaryKey.CreationTime.Add(time.Duration(*selfSig.KeyLifetimeSecs) * time.Second)
	}
	return expiry
}

func populateHash(hashFunc crypto.Hash, msg []byte) (hash.Hash, error) {
	h := hashFunc.New()
	if _, err := h.Write(msg); err != nil {
		return nil, err
	}
	return h, nil
}

// readArmoredSign read an armored signature block with the given type. https://sourcegraph.com/github.com/golang/crypto/-/blob/openpgp/read.go#L24:6-24:17
func readArmoredSign(r io.Reader) (body io.Reader, err error) {
	block, err := armor.Decode(r)
	if err != nil {
		return
	}
	if block.Type != openpgp.SignatureType {
		return nil, fmt.Errorf("expected '" + openpgp.SignatureType + "', got: " + block.Type)
	}
	return block.Body, nil
}

func extractSignature(s string) (*packet.Signature, error) {
	r, err := readArmoredSign(strings.NewReader(s))
	if err != nil {
		return nil, fmt.Errorf("Failed to read signature armor")
	}
	p, err := packet.Read(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to read signature packet")
	}
	sig, ok := p.(*packet.Signature)
	if !ok {
		return nil, fmt.Errorf("Packet is not a signature")
	}
	return sig, nil
}
