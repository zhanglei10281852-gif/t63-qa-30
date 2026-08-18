package idempotency

import (
	"context"
	"time"
)

type Record struct {
	Scope     string
	Key       string
	Status    int
	Response  []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Store interface {
	Find(context.Context, string, string, time.Time) (Record, bool, error)
	Save(context.Context, Record) error
}
