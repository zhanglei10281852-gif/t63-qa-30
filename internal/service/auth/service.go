package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/operator"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/repository"
)

type Service struct {
	Store repository.Store
	Clock clock.Clock
	IDs   identity.Generator
	TTL   time.Duration
}

type LoginResult struct {
	Token     string            `json:"token"`
	ExpiresAt time.Time         `json:"expires_at"`
	Operator  operator.Operator `json:"operator"`
}

func (s Service) Bootstrap(ctx context.Context, username, password, displayName string) error {
	username = normalizeUsername(username)
	if username == "" || password == "" {
		return fmt.Errorf("bootstrap username and password are required")
	}
	if _, err := s.Store.GetOperatorByUsername(ctx, username); err == nil {
		return nil
	} else if !isNotFound(err) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if displayName == "" {
		displayName = "Operations Administrator"
	}
	now := s.Clock.Now()
	return s.Store.SaveOperator(ctx, operator.Operator{ID: s.IDs.NewID("operator"), Username: username, DisplayName: displayName, Role: "administrator", Status: operator.Active, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now})
}

func (s Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	value, err := s.Store.GetOperatorByUsername(ctx, normalizeUsername(username))
	if err != nil || !value.CanLogin() || bcrypt.CompareHashAndPassword([]byte(value.PasswordHash), []byte(password)) != nil {
		return LoginResult{}, apperror.Unauthorized(errors.New("invalid username or password"))
	}
	token, err := randomToken()
	if err != nil {
		return LoginResult{}, apperror.Wrap("generate session", err)
	}
	now := s.Clock.Now()
	expires := now.Add(s.ttl())
	if err := s.Store.SaveSession(ctx, operator.Session{TokenHash: tokenHash(token), OperatorID: value.ID, ExpiresAt: expires, CreatedAt: now}); err != nil {
		return LoginResult{}, apperror.Wrap("save session", err)
	}
	value.PasswordHash = ""
	return LoginResult{Token: token, ExpiresAt: expires, Operator: value}, nil
}

func (s Service) Authenticate(ctx context.Context, token string) (operator.Operator, error) {
	if strings.TrimSpace(token) == "" {
		return operator.Operator{}, apperror.Unauthorized(errors.New("authentication required"))
	}
	session, err := s.Store.GetSession(ctx, tokenHash(token))
	if err != nil || !session.ValidAt(s.Clock.Now()) {
		return operator.Operator{}, apperror.Unauthorized(errors.New("session is invalid or expired"))
	}
	value, err := s.Store.GetOperator(ctx, session.OperatorID)
	if err != nil || !value.CanLogin() {
		return operator.Operator{}, apperror.Unauthorized(errors.New("operator is unavailable"))
	}
	return value, nil
}

func (s Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.Store.RevokeSession(ctx, tokenHash(token), s.Clock.Now())
}

func (s Service) ttl() time.Duration {
	if s.TTL <= 0 {
		return 12 * time.Hour
	}
	return s.TTL
}

func normalizeUsername(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func randomToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func isNotFound(err error) bool {
	var app *apperror.AppError
	return errors.As(err, &app) && app.Code == apperror.CodeNotFound
}
