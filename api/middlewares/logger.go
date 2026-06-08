package middlewares

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/logger"
)

type Logger interface {
	Middleware() gin.HandlerFunc
}

type LoggerImpl struct {
	logger logger.Logger
}

func NewLoggerMiddleware(logger *logger.Logger) (Logger, error) {
	return &LoggerImpl{
		logger: *logger,
	}, nil
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

func (l LoggerImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		l.logger.Info("request received", "method", c.Request.Method, "host", c.Request.Host, "path", c.Request.URL.Path)
		l.logger.Debug("request details", "query", sanitizeQuery(c.Request.URL.RawQuery), "headers", sanitizeHeaders(c.Request.Header))

		c.Next()
	}
}
