package catalog

import (
	"context"
	"marketplace/gen/catalogpb"
)

type Client struct {
	client catalogpb.CatalogServiceClient
}

func NewClient(client catalogpb.CatalogServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) Reserve(ctx context.Context, listingID string, qty int) error {
	_, err := c.client.Reserve(ctx, &catalogpb.ReserveCatalogRequest{
		ListingId: listingID,
		Qty:       int32(qty),
	})
	return err
}
