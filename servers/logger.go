package servers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggerService provides administration capabilities for remote SPIRE server logging.
type LoggerService struct {
	ctx *ServerContext
}

// NewLoggerService creates a new LoggerService bound to the provided ServerContext.
func NewLoggerService(ctx *ServerContext) *LoggerService {
	return &LoggerService{ctx: ctx}
}

// GetLogLevel retrieves the active log level of the SPIRE server.
func (l *LoggerService) GetLogLevel(ctx context.Context) (string, error) {
	return "", status.Errorf(codes.Unimplemented, "GetLogLevel is not yet implemented")
}

// SetLogLevel dynamically alters the log level of the SPIRE server.
func (l *LoggerService) SetLogLevel(ctx context.Context, level string) error {
	return status.Errorf(codes.Unimplemented, "SetLogLevel is not yet implemented")
}

// ResetLogLevel resets the SPIRE server log level to its default configuration.
func (l *LoggerService) ResetLogLevel(ctx context.Context) error {
	return status.Errorf(codes.Unimplemented, "ResetLogLevel is not yet implemented")
}
