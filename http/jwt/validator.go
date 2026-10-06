package jwt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/datarhei/core/v16/http/api"
	"github.com/datarhei/core/v16/http/handler/util"
	"github.com/datarhei/core/v16/http/jwt/jwks"
	"github.com/datarhei/core/v16/internal/secret"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type Validator interface {
	String() string

	// Validate returns true if it identified itself as validator for
	// that request. False if it doesn't handle this request. The string
	// is the username. An error is only returned if it identified itself
	// as validator but there was an error during validation.
	Validate(c echo.Context) (bool, string, error)
	Cancel()
}

type localValidator struct {
	username string
	password string
}

func NewLocalValidator(username, password string) (Validator, error) {
	v := &localValidator{
		username: username,
		password: password,
	}

	return v, nil
}

func (v *localValidator) String() string {
	return "localjwt"
}

func (v *localValidator) Validate(c echo.Context) (bool, string, error) {
	var login api.Login

	if err := util.ShouldBindJSON(c, &login); err != nil {
		return false, "", nil
	}

	usernameOK := secret.Equal(login.Username, v.username)
	passwordOK := secret.Equal(login.Password, v.password)

	if !usernameOK || !passwordOK {
		return true, "", fmt.Errorf("invalid username or password")
	}

	return true, v.username, nil
}

func (v *localValidator) Cancel() {}

type auth0Validator struct {
	domain   string
	issuer   string
	audience string
	clientID string
	users    []string
	certs    jwks.JWKS
}

func NewAuth0Validator(domain, audience, clientID string, users []string) (Validator, error) {
	v := &auth0Validator{
		domain:   domain,
		issuer:   "https://" + domain + "/",
		audience: audience,
		clientID: clientID,
		users:    users,
	}

	url := v.issuer + ".well-known/jwks.json"
	certs, err := jwks.NewFromURL(url, jwks.Config{})
	if err != nil {
		return nil, err
	}

	v.certs = certs

	return v, nil
}

func (v auth0Validator) String() string {
	return fmt.Sprintf("auth0 domain=%s audience=%s clientid=%s", v.domain, v.audience, v.clientID)
}

func (v *auth0Validator) Validate(c echo.Context) (bool, string, error) {
	// Look for an Auth header
	values := c.Request().Header.Values("Authorization")
	prefix := "Bearer "

	auth := ""
	for _, value := range values {
		if !strings.HasPrefix(value, prefix) {
			continue
		}

		auth = value[len(prefix):]

		break
	}

	if len(auth) == 0 {
		return false, "", nil
	}

	p := &jwtgo.Parser{}
	token, _, err := p.ParseUnverified(auth, jwtgo.MapClaims{})
	if err != nil {
		return false, "", nil
	}

	var issuer string
	if claims, ok := token.Claims.(jwtgo.MapClaims); ok {
		if iss, ok := claims["iss"]; ok {
			issuer = iss.(string)
		}
	}

	if issuer != v.issuer {
		return false, "", nil
	}

	token, err = jwtgo.Parse(auth, v.keyFunc)
	if err != nil {
		return true, "", err
	}

	if !token.Valid {
		return true, "", fmt.Errorf("invalid token")
	}

	var subject string
	if claims, ok := token.Claims.(jwtgo.MapClaims); ok {
		if sub, ok := claims["sub"]; ok {
			subject = sub.(string)
		}
	}

	return true, subject, nil
}

func (v *auth0Validator) keyFunc(token *jwtgo.Token) (any, error) {
	// Verify 'aud' claim against the configured audience. GetAudience only
	// parses the claim, so the value has to be checked explicitly.
	aud, err := token.Claims.GetAudience()
	if err != nil {
		return nil, fmt.Errorf("invalid audience: %w", err)
	}

	if !slices.Contains([]string(aud), v.audience) {
		return nil, fmt.Errorf("invalid audience")
	}

	// Verify 'iss' claim against the configured issuer
	iss, err := token.Claims.GetIssuer()
	if err != nil {
		return nil, fmt.Errorf("invalid issuer: %w", err)
	}

	if iss != v.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Verify 'sub' claim
	sub, err := token.Claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("invalid subject: %w", err)
	}

	found := slices.Contains(v.users, sub)

	if !found {
		return nil, fmt.Errorf("user not allowed")
	}

	// If the token carries an authorized party / client id, verify it against
	// the configured client id.
	if len(v.clientID) != 0 {
		if claims, ok := token.Claims.(jwtgo.MapClaims); ok {
			if azp, ok := claims["azp"].(string); ok && azp != v.clientID {
				return nil, fmt.Errorf("invalid authorized party")
			}

			if cid, ok := claims["client_id"].(string); ok && cid != v.clientID {
				return nil, fmt.Errorf("invalid client id")
			}
		}
	}

	// find the key
	if _, ok := token.Header["kid"]; !ok {
		return nil, fmt.Errorf("kid not found")
	}

	kid := token.Header["kid"].(string)

	key, err := v.certs.Key(kid)
	if err != nil {
		return nil, fmt.Errorf("no cert for kid found: %w", err)
	}

	// find algorithm
	if _, ok := token.Header["alg"]; !ok {
		return nil, fmt.Errorf("kid not found")
	}

	alg := token.Header["alg"].(string)

	if key.Alg() != alg {
		return nil, fmt.Errorf("signing method doesn't match")
	}

	// get the public key
	publicKey, err := key.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	return publicKey, nil
}

func (v *auth0Validator) Cancel() {
	v.certs.Cancel()
}
