// Copyright (c) 2019-2020 SpyderMix Ltd.

// Package jsonb provides a custom []byte to JSON encoding.
package pof

import (
	"crypto/ed25519"
	"encoding/base64"
)

var (
	// common encoding
	e = base64.RawURLEncoding

	// common stringifying func
	stringify = func(b []byte) string { return e.EncodeToString(b) }

	// common unstringifying func
	unstringify = func(s string) ([]byte, error) { return e.DecodeString(s) }
)

// Type B is for (un)marshaling regular []bytes and ed25519 signatures.
type B []byte

func (t B) MarshalText() ([]byte, error)        { return []byte(stringify(t)), nil }
func (t *B) UnmarshalText(b []byte) (err error) { *t, err = unstringify(string(b)); return }
func (t B) T() []byte                           { return t }
func (t B) String() string                      { return stringify(t) }

// Type SK is for (un)marshaling ed25519 private keys.
type SK ed25519.PrivateKey

func (t SK) MarshalText() ([]byte, error)        { return []byte(stringify(t)), nil }
func (t *SK) UnmarshalText(b []byte) (err error) { *t, err = unstringify(string(b)); return }
func (t SK) T() ed25519.PrivateKey               { return ed25519.PrivateKey(t) }
func (t SK) String() string                      { return stringify(t) }

// Type PK is for (un)marshaling ed25519 public keys.
type PK ed25519.PublicKey

func (t PK) MarshalText() ([]byte, error)        { return []byte(stringify(t)), nil }
func (t *PK) UnmarshalText(b []byte) (err error) { *t, err = unstringify(string(b)); return }
func (t PK) T() ed25519.PublicKey                { return ed25519.PublicKey(t) }
func (t PK) String() string                      { return stringify(t) }
