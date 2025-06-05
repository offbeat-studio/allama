package dbutils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogLevel type
type LogLevel string

const (
	// INFO level
	INFO LogLevel = "INFO"
	// ERROR level
	ERROR LogLevel = "ERROR"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string      `json:"timestamp"`
	Level     LogLevel    `json:"level"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

// Logger struct
type Logger struct {
	logDir string
}

// NewLogger creates a new logger instance
func NewLogger(logDir string) *Logger {
	return &Logger{logDir: logDir}
}

// Log writes a log entry to a daily log file
func (l *Logger) Log(level LogLevel, message string, data interface{}) error {
	now := time.Now()
	logFileName := fmt.Sprintf("%s/allama-%s.log", l.logDir, now.Format("2006-01-02"))
	entry := LogEntry{
		Timestamp: now.Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Data:      data,
	}

	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening log file: %w", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	if err := encoder.Encode(entry); err != nil {
		return fmt.Errorf("error encoding log entry: %w", err)
	}

	return nil
}

// LogRequest logs request details
func (l *Logger) LogRequest(method, path string, headers map[string][]string, body interface{}) error {
	data := map[string]interface{}{
		"method":  method,
		"path":    path,
		"headers": headers,
		"body":    body,
	}
	return l.Log(INFO, "Request", data)
}

// LogResponse logs response details
func (l *Logger) LogResponse(statusCode int, body interface{}) error {
	data := map[string]interface{}{
		"statusCode": statusCode,
		"body":       body,
	}
	return l.Log(INFO, "Response", data)
}

// LogError logs error details
func (l *Logger) LogError(message string, err error) error {
	data := map[string]interface{}{
		"error": err.Error(),
	}
	return l.Log(ERROR, message, data)
}

// EnsureLogDirExists checks if the log directory exists and creates it if not
func EnsureLogDirExists(logDir string) error {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("error creating log directory: %w", err)
		}
	}
	return nil
}

// SetOutputToNil prevents default log output to console
func SetOutputToNil() {
	log.SetOutput(io.Discard)
}
