package handler

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	started := time.Now()
	response, err := handler(ctx, req)
	code := status.Code(err)
	level := "INFO"
	if code == codes.Internal || code == codes.Unknown {
		level = "ERROR"
	}
	if err != nil {
		log.Printf("%s: gRPC request %q for %s completed with %s in %d ms; error: %q", level, requestID(ctx), info.FullMethod, code, time.Since(started).Milliseconds(), err.Error())
	} else {
		log.Printf("%s: gRPC request %q for %s completed with %s in %d ms", level, requestID(ctx), info.FullMethod, code, time.Since(started).Milliseconds())
	}
	return response, err
}

func requestID(ctx context.Context) string {
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		if values := incoming.Get("x-request-id"); len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
