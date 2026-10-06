package jwt

import (
	"testing"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// newTestToken creates an unsigned token with the given claims. The signature
// is irrelevant here because keyFunc is called before the signature is
// verified.
func newTestToken(claims jwtgo.MapClaims) *jwtgo.Token {
	return jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, claims)
}

func TestAuth0ValidatorKeyFunc(t *testing.T) {
	v := &auth0Validator{
		domain:   "example.auth0.com",
		issuer:   "https://example.auth0.com/",
		audience: "my-api",
		clientID: "my-client",
		users:    []string{"auth0|user"},
	}

	// A token for a different audience must be rejected.
	_, err := v.keyFunc(newTestToken(jwtgo.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": "other-api",
		"sub": "auth0|user",
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid audience")

	// A token without an audience must be rejected.
	_, err = v.keyFunc(newTestToken(jwtgo.MapClaims{
		"iss": "https://example.auth0.com/",
		"sub": "auth0|user",
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid audience")

	// A token from a different issuer must be rejected.
	_, err = v.keyFunc(newTestToken(jwtgo.MapClaims{
		"iss": "https://evil.example.com/",
		"aud": "my-api",
		"sub": "auth0|user",
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid issuer")

	// A user that is not on the allow list must be rejected.
	_, err = v.keyFunc(newTestToken(jwtgo.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": "my-api",
		"sub": "auth0|other",
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "user not allowed")

	// A token from a different authorized party must be rejected.
	_, err = v.keyFunc(newTestToken(jwtgo.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": "my-api",
		"sub": "auth0|user",
		"azp": "other-client",
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid authorized party")
}
