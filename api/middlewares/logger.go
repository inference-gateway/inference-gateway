package middlewares

import (
	"net/http"
	"net/url"
	"strings"

	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

type LoggerMiddleware struct {
	logger logger.Logger
}

func NewLoggerMiddleware(logger *logger.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: *logger,
	}
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "authorization") ||
		strings.Contains(k, "cookie") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password") ||
		strings.Contains(k, "api-key") ||
		strings.Contains(k, "apikey")
}

func sanitizeHeaders(headers map[string][]string) map[string][]string {
	sanitized := make(map[string][]string, len(headers))
	for key := range headers {
		sanitized[key] = []string{"[REDACTED]"}
	}
	return sanitized
}

func sanitizeQuery(rawQuery string) map[string][]string {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return map[string][]string{}
	}

	sanitized := make(map[string][]string, len(values))
	for key, vals := range values {
		if isSensitiveKey(key) {
			sanitized[key] = []string{"[REDACTED]"}
			continue
		}
		sanitized[key] = vals
	}
	return sanitized
}

func (l *LoggerMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.logger.Info("request received", "method", r.Method, "host", r.Host, "path", r.URL.Path)
			l.logger.Debug("request details", "query", sanitizeQuery(r.URL.RawQuery), "headers", sanitizeHeaders(r.Header))

			next.ServeHTTP(w, r)
		})
	}
}
