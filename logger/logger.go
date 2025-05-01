package logger

import (
	"errors"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:generate mockgen -source=logger.go -destination=../tests/mocks/logger.go -package=mocks
type Logger interface {
	Info(message string, fields ...interface{})
	Debug(message string, fields ...interface{})
	Error(message string, err error, fields ...interface{})
	Fatal(message string, err error, fields ...interface{})
}

type LoggerZapImpl struct {
	env    string
	logger *zap.Logger
}

// NoOpLogger is a logger implementation that discards all logs
// This is useful for testing to prevent logs from cluttering test output
type NoOpLogger struct{}

func (l *NoOpLogger) Info(message string, fields ...interface{})             {}
func (l *NoOpLogger) Debug(message string, fields ...interface{})            {}
func (l *NoOpLogger) Error(message string, err error, fields ...interface{}) {}
func (l *NoOpLogger) Fatal(message string, err error, fields ...interface{}) {}

// NewNoOpLogger returns a logger that discards all logs
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}

// isTestMode checks if the code is running as part of tests
func isTestMode() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}

// NewLogger initializes a logger
func NewLogger(env string) (Logger, error) {
	if isTestMode() {
		return NewNoOpLogger(), nil
	}

	var cfg zap.Config
	if env == "development" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	zapLogger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return &LoggerZapImpl{
		env:    env,
		logger: zapLogger,
	}, nil
}

func (l *LoggerZapImpl) Info(message string, fields ...interface{}) {
	l.logger.Info(message, parseFields(fields...)...)
}

func (l *LoggerZapImpl) Debug(message string, fields ...interface{}) {
	if l.env == "development" {
		l.logger.Debug(message, parseFields(fields...)...)
	}
}

func (l *LoggerZapImpl) Error(message string, err error, fields ...interface{}) {
	if err == nil {
		l.logger.Error(message, parseFields(fields...)...)
		return
	}
	fields = append(fields, "error", err.Error())
	l.logger.Error(message, parseFields(fields...)...)
}

func (l *LoggerZapImpl) Fatal(message string, err error, fields ...interface{}) {
	if err == nil {
		err = errors.New("unknown error")
	}
	fields = append(fields, "error", err.Error())
	l.logger.Fatal(message, parseFields(fields...)...)
}

func parseFields(kv ...interface{}) []zap.Field {
	var fields []zap.Field
	for i := 0; i < len(kv); i += 2 {
		if i+1 < len(kv) {
			key, ok := kv[i].(string)
			if !ok {
				continue
			}
			val := kv[i+1]
			fields = append(fields, zap.Any(key, val))
		}
	}
	return fields
}
