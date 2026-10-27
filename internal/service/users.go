package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"taskservice/internal/domain"
)

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type LoginResult struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254 && strings.Contains(email, "@")
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (domain.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if !validEmail(in.Email) {
		return domain.User{}, domain.Invalid("invalid email")
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return domain.User{}, domain.Invalid("password must contain 8 to 72 bytes")
	}
	if !textValid(in.Name, 100) {
		return domain.User{}, domain.Invalid("name must contain 1 to 100 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}
	u := domain.User{Email: in.Email, Name: in.Name, PasswordHash: string(hash), CreatedAt: now()}
	err = s.store.CreateUser(ctx, &u)
	return u, err
}

// A fixed bcrypt hash keeps nonexistent-account checks comparable to real checks.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (s *Service) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(in.Email) || len(in.Password) > 72 || in.Password == "" {
		return LoginResult{}, domain.ErrUnauthorized
	}
	u, err := s.store.UserByEmail(ctx, in.Email)
	if errors.Is(err, domain.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(in.Password))
		return LoginResult{}, domain.ErrUnauthorized
	}
	if err != nil {
		return LoginResult{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		return LoginResult{}, domain.ErrUnauthorized
	}
	token, err := s.tokens.Issue(u.ID)
	return LoginResult{AccessToken: token, TokenType: "Bearer"}, err
}
