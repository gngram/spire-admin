package trace

import (
	"log"
)

// Error logs an error message with the associated error.
func Error(msg string, err error) {
	if err != nil {
		log.Printf("[ERROR] %s: %v\n", msg, err)
	}
}

// Info logs an informational message.
func Info(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[INFO] "+msg+"\n", args...)
	} else {
		log.Printf("[INFO] %s\n", msg)
	}
}

// Warn logs a warning message.
func Warn(msg string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[WARN] "+msg+"\n", args...)
	} else {
		log.Printf("[WARN] %s\n", msg)
	}
}
