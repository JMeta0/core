package jwt

import (
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestParseTokenPinsSigningMethod(t *testing.T) {
	j := &jwt{
		secret: []byte("secret"),
	}

	parse := j.parseToken("access")

	// A token signed with a different HMAC algorithm must be rejected even
	// though the same key is used.
	token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS512, jwtgo.MapClaims{
		"usefor": "access",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})

	signed, err := token.SignedString(j.secret)
	require.NoError(t, err)

	_, err = parse(nil, signed)
	require.Error(t, err)

	// A valid HS256 token must be accepted.
	token = jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, jwtgo.MapClaims{
		"usefor": "access",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})

	signed, err = token.SignedString(j.secret)
	require.NoError(t, err)

	_, err = parse(nil, signed)
	require.NoError(t, err)

	// A token that is not valid for the access use must be rejected.
	token = jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, jwtgo.MapClaims{
		"usefor": "refresh",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})

	signed, err = token.SignedString(j.secret)
	require.NoError(t, err)

	_, err = parse(nil, signed)
	require.Error(t, err)
}

func TestAttemptLimiter(t *testing.T) {
	l := newAttemptLimiter(3, time.Minute)

	require.True(t, l.allow("a"))
	require.True(t, l.allow("a"))
	require.True(t, l.allow("a"))
	require.False(t, l.allow("a"))

	// A different key is not affected.
	require.True(t, l.allow("b"))

	// Expired events are dropped.
	l = newAttemptLimiter(1, time.Millisecond)
	require.True(t, l.allow("c"))
	require.False(t, l.allow("c"))
	time.Sleep(5 * time.Millisecond)
	require.True(t, l.allow("c"))
}
