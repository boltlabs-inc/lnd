// Copyright (c) 2019-2020 SpyderMix Ltd.

// Package nonce is used to create nonces.
package pof

import (
	"crypto/rand"
	"encoding/hex"
)

// New generates a nonce (random hex sequence) of size n.
func NewNonce(n int) (string, error) {
	byten := hex.DecodedLen(n) + 1
	b := make([]byte, byten)

	_, err := rand.Read(b)

	if err != nil {
		return "", err
	}

	h := hex.EncodeToString(b)
	return h[:n], nil
}
