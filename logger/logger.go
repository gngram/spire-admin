package logger

import (
	"log"
	"strings"
)

// Level represents the logging level.
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var currentLevel = InfoLevel

// SetLevel sets the global logging level.
func SetLevel(l Level) {
	currentLevel = l
}

// SetLevelString sets the global logging level using a string name.
func SetLevelString(levelStr string) {
	switch strings.ToLower(levelStr) {
	case "debug":
		SetLevel(DebugLevel)
	case "info":
		SetLevel(InfoLevel)
	case "warn", "warning":
		SetLevel(WarnLevel)
	case "error":
		SetLevel(ErrorLevel)
	}
}

// Error logs an error message with the associated error.
func Error(msg string, args ...interface{}) {
	if currentLevel <= ErrorLevel {
		if len(args) > 0 {
			log.Printf("[ERROR] "+msg+"\n", args...)
		} else {
			log.Printf("[ERROR] %s\n", msg)
		}
	}
}

// Info logs an informational message.
func Info(msg string, args ...interface{}) {
	if currentLevel <= InfoLevel {
		if len(args) > 0 {
			log.Printf("[INFO] "+msg+"\n", args...)
		} else {
			log.Printf("[INFO] %s\n", msg)
		}
	}
}

// Warn logs a warning message.
func Warn(msg string, args ...interface{}) {
	if currentLevel <= WarnLevel {
		if len(args) > 0 {
			log.Printf("[WARN] "+msg+"\n", args...)
		} else {
			log.Printf("[WARN] %s\n", msg)
		}
	}
}

// Debug logs a debug message.
func Debug(msg string, args ...interface{}) {
	if currentLevel <= DebugLevel {
		if len(args) > 0 {
			log.Printf("[DEBUG] "+msg+"\n", args...)
		} else {
			log.Printf("[DEBUG] %s\n", msg)
		}
	}
}
