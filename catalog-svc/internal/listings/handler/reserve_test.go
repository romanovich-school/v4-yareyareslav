package handler

import (
	"context"
	"marketplace/catalog-svc/internal/listings/dto"
	"marketplace/gen/catalogpb"
	"testing"

	"google.golang.org/protobuf/proto"
)

type reserveServiceStub struct {
	Service
	reserve func(context.Context, dto.ReserveListingRequest) (*dto.ReserveListingResponse, error)
}

func (s reserveServiceStub) Reserve(ctx context.Context, request dto.ReserveListingRequest) (*dto.ReserveListingResponse, error) {
	return s.reserve(ctx, request)
}

func TestReserveInt64RoundTrip(t *testing.T) {
	qty, remaining := int64(1<<40), int64(1<<41)
	request := &catalogpb.ReserveCatalogRequest{ListingId: "listing", Qty: qty}
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded := &catalogpb.ReserveCatalogRequest{}
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatal(err)
	}
	h := NewGRPCHandler(reserveServiceStub{reserve: func(ctx context.Context, request dto.ReserveListingRequest) (*dto.ReserveListingResponse, error) {
		if request.ListingID != "listing" || request.Qty != qty {
			t.Fatalf("unexpected request: %+v", request)
		}
		return &dto.ReserveListingResponse{Remaining: remaining}, nil
	}})
	response, err := h.Reserve(context.Background(), decoded)
	if err != nil {
		t.Fatal(err)
	}
	data, err = proto.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	decodedResponse := &catalogpb.ReserveCatalogResponse{}
	if err := proto.Unmarshal(data, decodedResponse); err != nil {
		t.Fatal(err)
	}
	if decodedResponse.Remaining != remaining {
		t.Fatalf("remaining = %d, want %d", decodedResponse.Remaining, remaining)
	}
}
