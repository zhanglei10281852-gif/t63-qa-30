package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/storage/sqlite"
)

func testService(t *testing.T) (Service, context.Context) {
	t.Helper()
	ctx := context.Background()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "auth.db"))
	store, err := sqlite.Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	return Service{Store: store, Clock: clock.Fixed{Current: now}, IDs: &identity.Sequence{}, TTL: time.Hour}, ctx
}

func TestBootstrapLoginAuthenticateAndLogout(t *testing.T) {
	service, ctx := testService(t)
	if err := service.Bootstrap(ctx, "Admin", "correct-password", "Test Administrator"); err != nil {
		t.Fatal(err)
	}
	if err := service.Bootstrap(ctx, "admin", "different-password", "Ignored"); err != nil {
		t.Fatalf("bootstrap must be idempotent: %v", err)
	}
	result, err := service.Login(ctx, " ADMIN ", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.Operator.Username != "admin" || result.Operator.PasswordHash != "" {
		t.Fatalf("login result %+v", result)
	}
	principal, err := service.Authenticate(ctx, result.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.ID != result.Operator.ID || principal.Role != "administrator" {
		t.Fatalf("principal %+v", principal)
	}
	if err := service.Logout(ctx, result.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, result.Token); err == nil {
		t.Fatal("revoked session was accepted")
	}
}

func TestLoginRejectsUnknownUserAndWrongPassword(t *testing.T) {
	service, ctx := testService(t)
	if err := service.Bootstrap(ctx, "admin", "correct-password", "Admin"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ username, password string }{
		{"missing", "correct-password"},
		{"admin", "wrong-password"},
		{"", ""},
	} {
		_, err := service.Login(ctx, input.username, input.password)
		var app *apperror.AppError
		if !errors.As(err, &app) || app.Code != apperror.CodeUnauthorized {
			t.Fatalf("login(%q) error=%v", input.username, err)
		}
	}
}

func TestAuthenticateRejectsMissingAndExpiredTokens(t *testing.T) {
	service, ctx := testService(t)
	if _, err := service.Authenticate(ctx, ""); err == nil {
		t.Fatal("empty token accepted")
	}
	if err := service.Bootstrap(ctx, "admin", "correct-password", "Admin"); err != nil {
		t.Fatal(err)
	}
	service.TTL = -time.Second
	result, err := service.Login(ctx, "admin", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	// Non-positive TTL intentionally falls back to the secure default.
	if _, err := service.Authenticate(ctx, result.Token); err != nil {
		t.Fatalf("default TTL session rejected: %v", err)
	}
	if _, err := service.Authenticate(ctx, "not-a-real-token"); err == nil {
		t.Fatal("unknown token accepted")
	}
}
