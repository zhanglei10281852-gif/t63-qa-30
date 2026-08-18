package operator

import "time"

type Status string

const (
	Active    Status = "active"
	Suspended Status = "suspended"
)

type Operator struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	Status       Status    `json:"status"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	TokenHash  string
	OperatorID string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

func (o Operator) CanLogin() bool { return o.Status == Active }
func (s Session) ValidAt(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}
