package logger

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Logger struct {
	env string
}

func New(env string) *Logger {
	return &Logger{env: env}
}

type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

func (l *Logger) log(level Level, msg string, ctx map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Severity:  string(level),
		Message:   msg,
		Context:   ctx,
	}

	if l.env == "production" {
		// Structured JSON for Cloud Logging
		json.NewEncoder(os.Stdout).Encode(entry)
	} else {
		// Human-readable for development
		log.Printf("[%s] %s %v", level, msg, ctx)
	}
}

func (l *Logger) Debug(msg string, ctx map[string]interface{}) {
	l.log(LevelDebug, msg, ctx)
}

func (l *Logger) Info(msg string, ctx map[string]interface{}) {
	l.log(LevelInfo, msg, ctx)
}

func (l *Logger) Warn(msg string, ctx map[string]interface{}) {
	l.log(LevelWarn, msg, ctx)
}

func (l *Logger) Error(msg string, ctx map[string]interface{}) {
	l.log(LevelError, msg, ctx)
}
