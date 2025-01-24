package logger

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{
			name:    "Development environment",
			env:     "development",
			wantErr: false,
		},
		{
			name:    "Production environment",
			env:     "production",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.env)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)

				// Type assertion to access internal fields
				loggerImpl, ok := logger.(*LoggerZapImpl)
				assert.True(t, ok)
				assert.Equal(t, tt.env, loggerImpl.env)
				assert.NotNil(t, loggerImpl.logger)
			}
		})
	}
}

func TestLoggerZapImpl_Methods(t *testing.T) {
	logger, err := NewLogger("development")
	assert.NoError(t, err)

	testCases := []struct {
		name    string
		method  func()
		message string
		err     error
		fields  []interface{}
	}{
		{
			name: "Info logging",
			method: func() {
				logger.Info("test info", "key1", "value1")
			},
			message: "test info",
			fields:  []interface{}{"key1", "value1"},
		},
		{
			name: "Debug logging",
			method: func() {
				logger.Debug("test debug", "key1", "value1")
			},
			message: "test debug",
			fields:  []interface{}{"key1", "value1"},
		},
		{
			name: "Error logging",
			method: func() {
				logger.Error("test error", errors.New("test error"), "key1", "value1")
			},
			message: "test error",
			err:     errors.New("test error"),
			fields:  []interface{}{"key1", "value1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This just verifies that the methods don't panic
			assert.NotPanics(t, func() {
				tc.method()
			})
		})
	}
}

func TestParseFields(t *testing.T) {
	tests := []struct {
		name   string
		input  []interface{}
		length int
	}{
		{
			name:   "Empty fields",
			input:  []interface{}{},
			length: 0,
		},
		{
			name:   "Key-value pairs",
			input:  []interface{}{"key1", "value1", "key2", 42},
			length: 2,
		},
		{
			name:   "Odd number of fields",
			input:  []interface{}{"key1", "value1", "key2"},
			length: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := parseFields(tt.input...)
			assert.Equal(t, tt.length, len(fields))
		})
	}
}
