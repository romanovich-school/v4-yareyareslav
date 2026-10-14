package catalog

import (
	"context"
	"marketplace/gen/catalogpb"

	"google.golang.org/grpc/metadata"
)

type Client struct {
	client catalogpb.CatalogServiceClient
}

func NewClient(client catalogpb.CatalogServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) Reserve(ctx context.Context, listingID string, qty int) error {
	if incoming, ok := metadata.FromIncomingContext(ctx); ok {
		if values := incoming.Get("x-request-id"); len(values) > 0 {
			outgoing, _ := metadata.FromOutgoingContext(ctx)
			outgoing = outgoing.Copy()
			outgoing.Set("x-request-id", values[0])
			ctx = metadata.NewOutgoingContext(ctx, outgoing)
		}
	}
	_, err := c.client.Reserve(ctx, &catalogpb.ReserveCatalogRequest{
		ListingId: listingID,
		Qty:       int64(qty),
	})
	return err
}
