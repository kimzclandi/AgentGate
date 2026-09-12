package gate

import (
	"context"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

type Identity struct {
	User   string `json:"user"`
	Tenant string `json:"tenant"`
	Role   string `json:"role"`
}
type Auth struct {
	Store  *Store
	DevKey []byte
	OIDC   *oidc.IDTokenVerifier
}

func (a *Auth) Token(user string) (string, error) {
	if len(a.DevKey) < 32 {
		return "", errors.New("dev authentication disabled")
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: user, Issuer: "agentgate-local-dev", Audience: jwt.ClaimStrings{"agentgate"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())})
	t.Header["kid"] = "dev-v1"
	return t.SignedString(a.DevKey)
}
func (a *Auth) Verify(ctx context.Context, raw string) (Identity, error) {
	var subject string
	if a.OIDC != nil {
		token, e := a.OIDC.Verify(ctx, raw)
		if e != nil {
			return Identity{}, ErrDenied
		}
		subject = token.Subject
	} else if len(a.DevKey) >= 32 {
		c := &jwt.RegisteredClaims{}
		_, e := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
			if t.Header["kid"] != "dev-v1" {
				return nil, ErrDenied
			}
			return a.DevKey, nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("agentgate-local-dev"), jwt.WithAudience("agentgate"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if e != nil {
			return Identity{}, ErrDenied
		}
		subject = c.Subject
	} else {
		return Identity{}, ErrDenied
	}
	if strings.TrimSpace(subject) == "" {
		return Identity{}, ErrDenied
	}
	var i Identity
	e := a.Store.DB.QueryRowContext(ctx, `SELECT id,tenant,role FROM principals WHERE id=? AND enabled=1`, subject).Scan(&i.User, &i.Tenant, &i.Role)
	if e != nil {
		return Identity{}, fmt.Errorf("%w", ErrDenied)
	}
	return i, nil
}
