// Package logger provides a thread-safe, structured JSON logging solution.
// It supports different log levels (INFO, ERROR, WARN, DEBUG) and optional structured data.
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log entry
// and defines the available log levels as constants.
type LogLevel string

const (
	Info  LogLevel = "INFO"  // Informational messages
	Error LogLevel = "ERROR" // Error conditions
	Warn  LogLevel = "WARN"  // Warning conditions
	Debug LogLevel = "DEBUG" // Debug-level messages
)

// LogEntry represents a single log entry with timestamp, level, message, and optional data
// The Data field uses json.RawMessage to store arbitrary JSON data efficiently
type LogEntry struct {
	Timestamp time.Time       `json:"timestamp"` // When the log entry was created (UTC)
	Level     LogLevel        `json:"level"`     // Log level (INFO, ERROR, WARN, DEBUG)
	Message   string          `json:"message"`   // The main log message
	Data      json.RawMessage `json:"data,omitempty"` // Optional structured data
}

// Logger is the main logger struct that handles writing log entries to a file
// It's safe for concurrent use from multiple goroutines
// Fields:
//   - file: The underlying file where logs are written
//   - encoder: JSON encoder for writing log entries
//   - mu: Mutex to ensure thread-safe writes
type Logger struct {
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
}

// Package-level variables for singleton pattern
var (
	singleton *Logger  // The single logger instance
	once      sync.Once // Used to ensure the logger is only initialized once
)

// NewLogger creates a new logger instance that writes to the specified file.
// It creates the log directory if it doesn't exist and opens the log file in append mode.
//
// Parameters:
//   - logPath: The full path to the log file
//
// Returns:
//   - *Logger: A new Logger instance
//   - error: Any error that occurred during logger creation
//
// Example:
//   logger, err := NewLogger("/var/log/myapp/app.log")
//   if err != nil {
//       log.Fatalf("Failed to create logger: %v", err)
//   }
//   defer logger.Close()
func NewLogger(logPath string) (*Logger, error) {
	// Ensure the directory exists with read/write/execute permissions for owner, read/execute for group/others
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open the log file in append mode, create it if it doesn't exist
	// O_APPEND - Append data to the file when writing
	// O_CREATE - Create the file if it doesn't exist
	// O_WRONLY - Open the file write-only
	// 0644 - File mode: read/write for owner, read-only for others
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a new Logger instance with the opened file and a new JSON encoder
	return &Logger{
		file:    file,                    // The log file
		encoder: json.NewEncoder(file),    // JSON encoder for writing log entries
	}, nil
}

// GetLogger returns a singleton instance of Logger.
// It ensures that only one Logger instance is created, even when called from multiple goroutines.
//
// Parameters:
//   - logPath: The full path to the log file
//
// Returns:
//   - *Logger: The singleton Logger instance
//   - error: Any error that occurred during logger initialization
//
// Note: Subsequent calls with different log paths after the first call will be ignored.
// The logger will continue to use the path from the first successful initialization.
func GetLogger(logPath string) (*Logger, error) {
	var err error

	// Use sync.Once to ensure the logger is only initialized once, even with concurrent calls
	once.Do(func() {
		singleton, err = NewLogger(logPath)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return singleton, nil
}

// Close closes the underlying log file.
// It's safe to call Close multiple times.
//
// Returns:
//   - error: Any error that occurred while closing the file
//
// Example:
//   err := logger.Close()
//   if err != nil {
//       log.Printf("Error closing log file: %v", err)
//   }
func (l *Logger) Close() error {
	// Check if the file is not nil before attempting to close it
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log is the internal method that handles the actual logging.
// It's not exported (starts with lowercase) as it's meant to be used by the package's public methods.
//
// Parameters:
//   - level: The severity level of the log entry
//   - message: The main log message
//   - data: Optional structured data to include in the log entry
//
// The method is safe for concurrent use by multiple goroutines.
func (l *Logger) log(level LogLevel, message string, data interface{}) {
	// Lock the mutex to ensure thread safety when writing to the log file
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create a new log entry with the current timestamp (in UTC)
	entry := LogEntry{
		Timestamp: time.Now().UTC(),  // Use UTC for consistent timestamping
		Level:     level,            // The log level (INFO, ERROR, etc.)
		Message:   message,          // The actual log message
	}

	// If additional data was provided, try to marshal it to JSON
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err == nil {
			// Store the raw JSON data in the log entry
			// Using json.RawMessage allows for efficient JSON handling
			entry.Data = json.RawMessage(jsonData)
		}
		// If marshaling fails, we just log the message without the data
		// In a production environment, you might want to handle this differently
	}

	// If we have a valid encoder, write the log entry as a JSON line
	if l.encoder != nil {
		// We ignore the error from Encode as there's not much we can do if logging fails
		_ = l.encoder.Encode(entry)
	}
}

// Info logs an informational message.
// Use this for general, non-critical information about the application's operation.
//
// Parameters:
//   - message: The main log message
//   - data: Optional structured data to include in the log entry
//
// Example:
//   logger.Info("Application started", map[string]interface{}{
//       "version": "1.0.0",
//       "environment": "production",
//   })
func (l *Logger) Info(message string, data interface{}) {
	l.log(Info, message, data)
}

// Error logs an error message along with error details.
// Use this for logging error conditions that need to be investigated.
//
// Parameters:
//   - message: A description of what went wrong
//   - err: The error that occurred
//   - data: Optional additional context about the error
//
// The error message will be included in the log entry's data.
// If data is nil, a new map will be created with the error.
// If data is a map, the error will be added to it with the key "error".
//
// Example:
//   if err := someOperation(); err != nil {
//       logger.Error("Failed to perform operation", err, map[string]interface{}{
//           "operation": "database update",
//           "retry_count": 3,
//       })
//   }
func (l *Logger) Error(message string, err error, data interface{}) {
	if err == nil {
		// If no error was provided, log the message as a warning
		l.log(Warn, message+" (no error provided)", data)
		return
	}

	// Initialize data if it's nil
	if data == nil {
		data = make(map[string]interface{})
	}

	// If data is a map, add the error to it
	if dataMap, ok := data.(map[string]interface{}); ok {
		// Don't overwrite existing error
		if _, exists := dataMap["error"]; !exists {
			dataMap["error"] = err.Error()
		}
	}

	l.log(Error, message, data)
}

// Warn logs a warning message.
// Use this for conditions that are not errors but might indicate potential issues.
//
// Parameters:
//   - message: The warning message
//   - data: Optional structured data to include in the log entry
//
// Example:
//   if len(users) == 0 {
//       logger.Warn("No active users found", map[string]interface{}{
//           "timeout": timeout,
//       })
//   }
func (l *Logger) Warn(message string, data interface{}) {
	l.log(Warn, message, data)
}

// Debug logs a debug message.
// Use this for detailed information that's only needed for debugging.
// These messages are typically disabled in production.
//
// Parameters:
//   - message: The debug message
//   - data: Optional structured data to include in the log entry
//
// Example:
//   logger.Debug("Processing request", map[string]interface{}{
//       "request_id": requestID,
//       "processing_time_ms": elapsed.Milliseconds(),
//   })
func (l *Logger) Debug(message string, data interface{}) {
	l.log(Debug, message, data)
}
