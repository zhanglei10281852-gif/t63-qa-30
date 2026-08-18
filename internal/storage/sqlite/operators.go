package sqlite

import (
	"context"
	"strings"
	"time"

	"sanitation-operations/internal/domain/operator"
)

const operatorColumns = "id, username, display_name, role, status, password_hash, created_at, updated_at"

func scanOperator(s scanner) (operator.Operator, error) {
	var value operator.Operator
	var created, updated string
	if err := s.Scan(&value.ID, &value.Username, &value.DisplayName, &value.Role, &value.Status, &value.PasswordHash, &created, &updated); err != nil {
		return operator.Operator{}, notFound(err)
	}
	value.CreatedAt, value.UpdatedAt = parseTime(created), parseTime(updated)
	return value, nil
}

func (s *queryStore) GetOperator(ctx context.Context, id string) (operator.Operator, error) {
	return scanOperator(s.e.QueryRowContext(ctx, "SELECT "+operatorColumns+" FROM operators WHERE id=?", id))
}

func (s *queryStore) GetOperatorByUsername(ctx context.Context, username string) (operator.Operator, error) {
	return scanOperator(s.e.QueryRowContext(ctx, "SELECT "+operatorColumns+" FROM operators WHERE username=?", username))
}

func (s *queryStore) SaveOperator(ctx context.Context, value operator.Operator) error {
	_, err := s.e.ExecContext(ctx, "INSERT INTO operators("+operatorColumns+") VALUES(?,?,?,?,?,?,?,?)", value.ID, value.Username, value.DisplayName, value.Role, value.Status, value.PasswordHash, formatTime(value.CreatedAt), formatTime(value.UpdatedAt))
	return databaseError(err)
}

func (s *queryStore) GetSession(ctx context.Context, tokenHash string) (operator.Session, error) {
	var value operator.Session
	var expires, created string
	var revoked *string
	err := s.e.QueryRowContext(ctx, "SELECT token_hash, operator_id, expires_at, created_at, revoked_at FROM operator_sessions WHERE token_hash=?", tokenHash).Scan(&value.TokenHash, &value.OperatorID, &expires, &created, &revoked)
	if err != nil {
		return operator.Session{}, notFound(err)
	}
	value.ExpiresAt, value.CreatedAt = parseTime(expires), parseTime(created)
	if revoked != nil && strings.TrimSpace(*revoked) != "" {
		stamp := parseTime(*revoked)
		value.RevokedAt = &stamp
	}
	return value, nil
}

func (s *queryStore) SaveSession(ctx context.Context, value operator.Session) error {
	_, err := s.e.ExecContext(ctx, "INSERT INTO operator_sessions(token_hash, operator_id, expires_at, created_at, revoked_at) VALUES(?,?,?,?,NULL)", value.TokenHash, value.OperatorID, formatTime(value.ExpiresAt), formatTime(value.CreatedAt))
	return databaseError(err)
}

func (s *queryStore) RevokeSession(ctx context.Context, tokenHash string, at time.Time) error {
	_, err := s.e.ExecContext(ctx, "UPDATE operator_sessions SET revoked_at=? WHERE token_hash=?", formatTime(at), tokenHash)
	return err
}
