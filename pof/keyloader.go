package pof

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

func LoadKey(fm FsDir, p ...string) (key ed25519.PrivateKey, err error) {
	var seed B
	err = fm.Get(&seed, p...)

	if err != nil {
		var jse *json.SyntaxError
		if errors.As(err, &jse) {
			// could be old unquoted format
			var b []byte
			b, err = ioutil.ReadFile(fm.Path("key.seed"))

			if err != nil {
				return
			}

			var n int
			dec := make([]byte, base64.RawURLEncoding.DecodedLen(len(b)))
			n, err = base64.RawURLEncoding.Decode(dec, b)

			if err != nil {
				return
			}

			// decoding succeeded, so it's a valid old format key
			// update the file
			seed = B(dec[:n])
			err = fm.Set(&seed, p...)

			if err != nil {
				return
			}
		}
	}

	size := len(seed.T())
	if size != ed25519.SeedSize {
		err = fmt.Errorf(
			"%s has invalid seed size: %d, should be %d",
			fm.Path(p...),
			size,
			ed25519.SeedSize,
		)
		return
	}

	key = ed25519.NewKeyFromSeed(seed.T())
	return
}