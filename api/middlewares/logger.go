package middlewares

import (
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

func (l LoggerImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		l.logger.Info("Request received", "method", c.Request.Method, "host", c.Request.Host, "path", c.Request.URL.Path)

		c.Next()
	}
}
