// Package secret provides helpers to compare secrets without leaking
// information through timing side channels.
package secret

import (
	"crypto/sha256"
	"crypto/subtle"
)

// Equal reports whether a and b are equal. Both values are hashed first so that
// the comparison itself always operates on fixed-size digests and therefore
// runs in constant time with respect to the contents of a and b.
func Equal(a, b string) bool {
	ad := sha256.Sum256([]byte(a))
	bd := sha256.Sum256([]byte(b))

	return subtle.ConstantTimeCompare(ad[:], bd[:]) == 1
}
