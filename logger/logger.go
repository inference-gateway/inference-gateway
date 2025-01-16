package logger

import (
	"encoding/json"
	"errors"
	"log"
	"os"
)

//go:generate mockgen -source=logger.go -destination=mocks/logger.go -package=mocks
type Logger interface {
	Info(message string, fields ...interface{})
	Debug(message string, fields ...interface{})
	Error(message string, err error, fields ...interface{})
}

type LoggerImpl struct {
	Environment string
}

type LogEntry struct {
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

func NewLogger(env string) *LoggerImpl {
	return &LoggerImpl{
		Environment: env,
	}
}

func (l *LoggerImpl) Info(message string, fields ...interface{}) {
	entry := LogEntry{
		Level:   "INFO",
		Message: message,
		Fields:  make(map[string]interface{}),
	}
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key, ok := fields[i].(string)
			if !ok {
				continue
			}
			entry.Fields[key] = fields[i+1]
		}
	}
	l.output(entry)
}

func (l *LoggerImpl) Debug(message string, fields ...interface{}) {
	if l.Environment == "development" {
		entry := LogEntry{
			Level:   "DEBUG",
			Message: message,
			Fields:  make(map[string]interface{}),
		}
		for i := 0; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				key, ok := fields[i].(string)
				if !ok {
					continue
				}
				entry.Fields[key] = fields[i+1]
			}
		}
		l.output(entry)
	}
}

func (l *LoggerImpl) Error(message string, err error, fields ...interface{}) {
	if err == nil {
		err = errors.New("unknown error")
	}
	entry := LogEntry{
		Level:   "ERROR",
		Message: message + ": " + err.Error(),
		Fields:  make(map[string]interface{}),
	}
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key, ok := fields[i].(string)
			if !ok {
				continue
			}
			entry.Fields[key] = fields[i+1]
		}
	}
	l.output(entry)
}

func (l *LoggerImpl) Fatal(message string, err error, fields ...interface{}) {
	if err == nil {
		err = errors.New("unknown error")
	}
	entry := LogEntry{
		Level:   "FATAL",
		Message: message + ": " + err.Error(),
		Fields:  make(map[string]interface{}),
	}
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key, ok := fields[i].(string)
			if !ok {
				continue
			}
			entry.Fields[key] = fields[i+1]
		}
	}
	l.output(entry)
	os.Exit(1)
}

func (l *LoggerImpl) output(entry LogEntry) {
	b, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}
	log.Println(string(b))
}
