package audit

import (
	"context"
	"encoding/json"
	"time"
)

type Event struct {
	ID         string
	ActorID    string
	EntityType string
	EntityID   string
	Action     string
	Result     string
	RequestID  string
	Metadata   map[string]any
	CreatedAt  time.Time
}

type Sink interface {
	Append(context.Context, Event) error
}

func Metadata(values map[string]any) string {
	if values == nil {
		return "{}"
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(data)
}
