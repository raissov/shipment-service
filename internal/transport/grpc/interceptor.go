package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// RequestIDInterceptor injects a unique request ID into each request context.
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		requestID := uuid.New().String()
		ctx = context.WithValue(ctx, requestIDKey, requestID)
		return handler(ctx, req)
	}
}

// LoggingInterceptor returns a gRPC unary server interceptor that logs requests.
func LoggingInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		event := log.Info()
		if code != codes.OK {
			event = log.Warn()
		}
		if code == codes.Internal || code == codes.Unknown {
			event = log.Error()
		}

		event.
			Str("method", info.FullMethod).
			Str("request_id", RequestIDFromContext(ctx)).
			Dur("duration", duration).
			Str("code", code.String()).
			Err(err).
			Msg("gRPC request")

		return resp, err
	}
}
