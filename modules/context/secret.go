// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// NewSecret creates a new secret
func NewSecret() (string, error) {
	return NewSecretWithLength(32)
}

// NewSecretWithLength creates a new secret for a given length
func NewSecretWithLength(length int64) (string, error) {
	return randomString(length)
}

func randomBytes(len int64) ([]byte, error) {
	b := make([]byte, len)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func randomString(len int64) (string, error) {
	b, err := randomBytes(len)
	return base64.URLEncoding.EncodeToString(b), err
}

// AesEncrypt encrypts text and given key with AES.
func AesEncrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

// AesDecrypt decrypts text and given key with AES.
func AesDecrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// EncryptSecret encrypts a string with given key into a hex string
func EncryptSecret(key, str string) (string, error) {
	keyHash := sha256.Sum256([]byte(key))
	plaintext := []byte(str)
	ciphertext, err := AesEncrypt(keyHash[:], plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts a previously encrypted hex string
func DecryptSecret(key, cipherhex string) (string, error) {
	keyHash := sha256.Sum256([]byte(key))
	ciphertext, err := base64.StdEncoding.DecodeString(cipherhex)
	if err != nil {
		return "", err
	}
	plaintext, err := AesDecrypt(keyHash[:], ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
