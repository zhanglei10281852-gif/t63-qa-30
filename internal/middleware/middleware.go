package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/security"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func RequestID(ids identity.Generator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = ids.NewID("req")
			}
			ctx := context.WithValue(r.Context(), requestIDKey, id)
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func RequestIDFrom(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.InfoContext(r.Context(), "request started", "method", r.Method, "target", security.SafeTarget(r), "client_ip", security.ClientIP(r), "request_id", RequestIDFrom(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if value := recover(); value != nil {
					logger.ErrorContext(r.Context(), "panic recovered", "panic", value, "stack", string(debug.Stack()), "request_id", RequestIDFrom(r.Context()))
					http.Error(w, `{"code":"internal_error","message":"internal server error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Chain(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for index := len(middleware) - 1; index >= 0; index-- {
		handler = middleware[index](handler)
	}
	return handler
}
