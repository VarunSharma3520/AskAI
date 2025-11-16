package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)


type FileLogger struct {
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
}

// NewFileLogger creates a new file logger
func NewFileLogger(logPath string) (*FileLogger, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open the log file in append mode, create it if it doesn't exist
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &FileLogger{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// Close closes the log file
func (l *FileLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a log entry
func (l *FileLogger) log(level LogLevel, message string, data interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
	}

	if data != nil {
		jsonData, err := json.Marshal(data)
		if err == nil {
			entry.Data = jsonData
		}
	}

	if l.encoder != nil {
		_ = l.encoder.Encode(entry)
	}
}

// Info logs an info message
func (l *FileLogger) Info(message string, data interface{}) {
	l.log(Info, message, data)
}

// Error logs an error message
func (l *FileLogger) Error(message string, err error, data interface{}) {
	if data == nil {
		data = map[string]interface{}{"error": err.Error()}
	} else if dataMap, ok := data.(map[string]interface{}); ok {
		dataMap["error"] = err.Error()
	}
	l.log(Error, message, data)
}

// Warn logs a warning message
func (l *FileLogger) Warn(message string, data interface{}) {
	l.log(Warn, message, data)
}

// Debug logs a debug message
func (l *FileLogger) Debug(message string, data interface{}) {
	l.log(Debug, message, data)
}
