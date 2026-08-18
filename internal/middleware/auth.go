package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"sanitation-operations/internal/domain/operator"
)

const operatorKey contextKey = "operator"

type TokenVerifier func(context.Context, string) (operator.Operator, error)

func Authentication(verify TokenVerifier, publicPaths ...string) func(http.Handler) http.Handler {
	public := make(map[string]bool, len(publicPaths))
	for _, path := range publicPaths {
		public[path] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || public[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			token := BearerToken(r)
			value, err := verify(r.Context(), token)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": "unauthorized", "message": "authentication required", "request_id": RequestIDFrom(r.Context())})
				return
			}
			ctx := context.WithValue(r.Context(), operatorKey, value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func BearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < 8 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

func OperatorFrom(ctx context.Context) (operator.Operator, bool) {
	value, ok := ctx.Value(operatorKey).(operator.Operator)
	return value, ok
}
