package security

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

var sensitiveQuery = map[string]bool{"token": true, "access_token": true, "password": true, "secret": true}

func SafeTarget(request *http.Request) string {
	copied := *request.URL
	values := copied.Query()
	for key := range values {
		if sensitiveQuery[strings.ToLower(key)] {
			values.Set(key, "[redacted]")
		}
	}
	copied.RawQuery = values.Encode()
	return copied.RequestURI()
}
func ClientIP(request *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0])
	if net.ParseIP(forwarded) != nil {
		return forwarded
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && net.ParseIP(host) != nil {
		return host
	}
	if net.ParseIP(request.RemoteAddr) != nil {
		return request.RemoteAddr
	}
	return "unknown"
}
func SafeURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return "invalid-url"
	}
	parsed.User = nil
	values := parsed.Query()
	for key := range values {
		if sensitiveQuery[strings.ToLower(key)] {
			values.Set(key, "[redacted]")
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}
