package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestLoggingInterceptor(t *testing.T) {
	tests := []struct {
		name          string
		incoming      metadata.MD
		outgoing      metadata.MD
		err           error
		wantRequestID string
		wantCode      codes.Code
		wantLevel     string
	}{
		{name: "success with request ID", incoming: metadata.Pairs("x-request-id", "request-1"), wantRequestID: "request-1", wantCode: codes.OK, wantLevel: "INFO"},
		{name: "no metadata", wantCode: codes.OK, wantLevel: "INFO"},
		{name: "missing request ID", incoming: metadata.Pairs("other", "value"), wantCode: codes.OK, wantLevel: "INFO"},
		{name: "first request ID used", incoming: metadata.Pairs("x-request-id", "first", "x-request-id", "second"), wantRequestID: "first", wantCode: codes.OK, wantLevel: "INFO"},
		{name: "outgoing metadata ignored", outgoing: metadata.Pairs("x-request-id", "outgoing"), wantCode: codes.OK, wantLevel: "INFO"},
		{name: "internal error", err: status.Error(codes.Internal, "database failed"), wantCode: codes.Internal, wantLevel: "ERROR"},
		{name: "unknown status", err: status.Error(codes.Unknown, "unknown failure"), wantCode: codes.Unknown, wantLevel: "ERROR"},
		{name: "plain error", err: errors.New("unexpected failure"), wantCode: codes.Unknown, wantLevel: "ERROR"},
		{name: "validation error", err: status.Error(codes.InvalidArgument, "invalid quantity"), wantCode: codes.InvalidArgument, wantLevel: "INFO"},
		{name: "unavailable", err: status.Error(codes.Unavailable, "catalog unavailable"), wantCode: codes.Unavailable, wantLevel: "INFO"},
		{name: "canceled", err: status.Error(codes.Canceled, "request canceled"), wantCode: codes.Canceled, wantLevel: "INFO"},
		{name: "deadline", err: status.Error(codes.DeadlineExceeded, "request timed out"), wantCode: codes.DeadlineExceeded, wantLevel: "INFO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var output bytes.Buffer
				previousOutput, previousFlags := log.Writer(), log.Flags()
				log.SetOutput(&output)
				log.SetFlags(0)
				defer func() {
					log.SetOutput(previousOutput)
					log.SetFlags(previousFlags)
				}()

				ctx := context.Background()
				if tt.incoming != nil {
					ctx = metadata.NewIncomingContext(ctx, tt.incoming)
				}
				if tt.outgoing != nil {
					ctx = metadata.NewOutgoingContext(ctx, tt.outgoing)
				}
				request := &struct{ Value string }{Value: "request"}
				response := &struct{ Value string }{Value: "response"}
				method := "/marketplace.orders.v1.OrdersService/Create"
				calls := 0
				handler := func(receivedCtx context.Context, receivedRequest any) (any, error) {
					calls++
					if receivedCtx != ctx || receivedRequest != request {
						t.Error("handler received a different context or request")
					}
					if output.Len() != 0 {
						t.Error("log was written before handler completed")
					}
					time.Sleep(17 * time.Millisecond)
					return response, tt.err
				}

				got, err := LoggingInterceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, handler)
				if got != response || err != tt.err {
					t.Errorf("response = %v, error = %v, want original response and error", got, err)
				}
				if calls != 1 {
					t.Errorf("handler calls = %d, want 1", calls)
				}

				want := fmt.Sprintf("%s: gRPC request %q for %s completed with %s in 17 ms", tt.wantLevel, tt.wantRequestID, method, tt.wantCode)
				if tt.err != nil {
					want += fmt.Sprintf("; error: %q", tt.err.Error())
				}
				if got := output.String(); got != want+"\n" {
					t.Errorf("log = %q, want %q", got, want+"\n")
				}
				if strings.Count(output.String(), "\n") != 1 {
					t.Error("expected exactly one log line")
				}
			})
		})
	}
}
