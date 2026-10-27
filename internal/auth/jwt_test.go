package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestTokens(t *testing.T) {
	secret := "test-secret-at-least-thirty-two-bytes"
	tokens := New(secret, time.Hour)
	raw, err := tokens.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := tokens.Verify(raw); err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	for _, claims := range []jwt.RegisteredClaims{
		{Issuer: "task-service", Subject: "42"},
		{Issuer: "task-service", Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))},
		{Issuer: "other", Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{Issuer: "task-service", Subject: "0", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	} {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tokens.Verify(raw); err == nil {
			t.Fatal("accepted invalid claims")
		}
	}
	if _, err := New("wrong-secret-but-at-least-32-bytes-long", time.Hour).Verify(raw); err == nil {
		t.Fatal("accepted wrong signature")
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Issuer: "task-service", Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tokens.Verify(none); err == nil {
		t.Fatal("accepted unsigned token")
	}
}
