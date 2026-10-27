package auth

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"taskservice/internal/domain"
)

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Tokens { return &Tokens{[]byte(secret), ttl} }

func (t *Tokens) Issue(userID int64) (string, error) {
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{Issuer: "task-service", Subject: strconv.FormatInt(userID, 10), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl))}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *Tokens) Verify(raw string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) { return t.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("task-service"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return 0, domain.ErrUnauthorized
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrUnauthorized
	}
	return id, nil
}
