// Copyright (c) 2019-2020 SpyderMix Ltd.

// Package pof provides a mechanism to generate proofs of funding.
package pof

import (
	"strconv"
	"strings"
	"time"
)

type T struct {
	Type       string  `json:"type,omitempty"`
	Expiration int64   `json:"expiration,omitempty"`
	Nonce      string  `json:"nonce,omitempty"`
	Signature  B `json:"signature,omitempty"`
}

func (t *T) Digest() string {
	return strings.Join([]string{
		t.Type,
		strconv.FormatInt(t.Expiration, 10),
		t.Nonce,
	}, ":")
}

func New(s Signer, poftype string, duration int64) (*T, error) {
	exp := time.Now().Unix() + duration
	nonce, err := NewNonce(18)

	if err != nil {
		return nil, err
	}

	pof := &T{
		Type:       poftype,
		Expiration: exp,
		Nonce:      nonce,
	}

	pof.Signature = s.Sign([]byte(pof.Digest()))
	return pof, nil
}

type SKActivationRequest struct {
	Pubkey PK `json:"pubkey,omitempty"`
	Pof    *T       `json:"pof,omitempty"`
}

func (t *T) IsExpiredAt(utime int64) bool {
	return t.Expiration <= utime
}
