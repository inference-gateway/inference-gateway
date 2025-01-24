package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry interface {
	Middleware() gin.HandlerFunc
}

type TelemetryImpl struct {
	cfg config.Config
	tp  otel.TracerProvider
}

func NewTelemetryMiddleware(cfg config.Config, tp otel.TracerProvider) (Telemetry, error) {
	return &TelemetryImpl{
		cfg: cfg,
		tp:  tp,
	}, nil
}

func (t *TelemetryImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !t.cfg.EnableTelemetry {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		_, span := t.tp.Tracer("inference-gateway").Start(ctx, "proxy-request")
		defer span.End()

		span.AddEvent("Proxying request", trace.WithAttributes(
			semconv.HTTPMethodKey.String(c.Request.Method),
			semconv.HTTPTargetKey.String(c.Request.URL.String()),
			semconv.HTTPRequestContentLengthKey.Int64(c.Request.ContentLength),
		))

		c.Next()
	}
}
