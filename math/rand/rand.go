package rand

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
)

// https://www.calhoun.io/creating-random-strings-in-go/

const (
	CharsetLetters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsetNumbers = "1234567890"
	CharsetSymbols = "#@+*%&/<>[]()=?!$.,:;-_"

	CharsetAlphanumeric = CharsetLetters + CharsetNumbers
	CharsetAll          = CharsetLetters + CharsetNumbers + CharsetSymbols
)

// StringWithCharset returns a cryptographically secure random string of the
// given length using characters from the given charset.
func StringWithCharset(length int, charset string) string {
	if length <= 0 || len(charset) == 0 {
		return ""
	}

	b := make([]byte, length)

	max := big.NewInt(int64(len(charset)))

	for i := range b {
		n, err := crand.Int(crand.Reader, max)
		if err != nil {
			// crypto/rand must not fail. If it does there is no safe way to
			// continue generating secrets, so fail loudly.
			panic(fmt.Sprintf("unable to read from crypto/rand: %s", err))
		}

		b[i] = charset[n.Int64()]
	}

	return string(b)
}

func StringLetters(length int) string {
	return StringWithCharset(length, CharsetLetters)
}

func StringNumbers(length int) string {
	return StringWithCharset(length, CharsetNumbers)
}

func StringAlphanumeric(length int) string {
	return StringWithCharset(length, CharsetAlphanumeric)
}

func String(length int) string {
	return StringWithCharset(length, CharsetAll)
}
