package listings

import "context"

type ListingsService struct{}

func NewListingsService() *ListingsService {
	return &ListingsService{}
}

func (s *ListingsService) FindAll(ctx context.Context) ([]Listing, error) {
	return []Listing{}, nil
}

func (s *ListingsService) FindOne(ctx context.Context, id int) (Listing, error) {
	return Listing{}, nil
}

func (s *ListingsService) CreateOne(ctx context.Context) (Listing, error) {
	return Listing{}, nil
}

func (s *ListingsService) PatchOne(ctx context.Context) (Listing, error) {
	return Listing{}, nil
}

func (s *ListingsService) DeleteOne(ctx context.Context, id int) (Listing, error) {
	return Listing{}, nil
}