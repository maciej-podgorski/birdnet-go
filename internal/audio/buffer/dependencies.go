package buffer

import (
	"log"
	"time"
)

// TimeProvider abstracts time-related operations
type TimeProvider interface {
	Now() time.Time
	Sleep(duration time.Duration)
}

// RealTimeProvider is the default implementation using actual time
type RealTimeProvider struct{}

func (t *RealTimeProvider) Now() time.Time {
	return time.Now()
}

func (t *RealTimeProvider) Sleep(d time.Duration) {
	time.Sleep(d)
}

// Logger abstracts logging operations
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// StandardLogger is the default implementation using standard log package
type StandardLogger struct{}

func (l *StandardLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}

func (l *StandardLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *StandardLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *StandardLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}
