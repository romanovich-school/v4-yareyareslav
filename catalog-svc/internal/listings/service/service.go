package service

import (
	"context"
	"fmt"
	"marketplace/catalog-svc/internal/listings/dto"
	"marketplace/catalog-svc/internal/listings/mapper"
	"marketplace/catalog-svc/internal/listings/model"
	"marketplace/catalog-svc/internal/listings/repository"
)

type Repository interface {
	Create(context.Context, model.Listing) (*model.Listing, error)
	Get(context.Context, string) (*model.Listing, error)
	List(context.Context, int, int) ([]model.Listing, int64, error)
	Update(context.Context, model.UpdateListing) (*model.Listing, error)
	Delete(context.Context, string, string) error
	Reserve(context.Context, string, int64) (int64, error)
	ApplyOrderCreated(context.Context, string, string, int64) (bool, error)
}

type CatalogService struct {
	repository Repository
}

func NewCatalogService(repository Repository) *CatalogService {
	return &CatalogService{
		repository: repository,
	}
}

func (s *CatalogService) Create(
	ctx context.Context,
	createDto dto.CreateListingRequest,
) (*dto.ListingResponse, error) {
	if err := validateCreate(createDto); err != nil {
		return nil, fmt.Errorf("service create: %w", err)
	}

	listing := model.Listing{
		Title:    createDto.Title,
		Price:    createDto.Price,
		SellerID: createDto.SellerID,
		Stock:    *createDto.Stock,
	}
	createdListing, err := s.repository.Create(ctx, listing)
	if err != nil {
		return nil, fmt.Errorf("service create: %w", err)
	}

	response := mapper.ToListingResponse(*createdListing)
	return &response, nil
}

func (s *CatalogService) Get(
	ctx context.Context,
	id string,
) (*dto.ListingResponse, error) {
	if !isValidUUID(id) {
		return nil, fmt.Errorf("service get: %w", repository.ErrNotFound)
	}

	listing, err := s.repository.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service get: %w", err)
	}

	response := mapper.ToListingResponse(*listing)
	return &response, nil
}

func (s *CatalogService) List(
	ctx context.Context,
	limit,
	offset int,
) (*dto.ListResponse, error) {
	listings, total, err := s.repository.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("service list: %w", err)
	}

	responseListings := make([]dto.ListingResponse, 0, len(listings))
	for _, listing := range listings {
		responseListings = append(
			responseListings,
			mapper.ToListingResponse(listing),
		)
	}
	return &dto.ListResponse{
		Listings: responseListings,
		Total:    total,
	}, nil
}

func (s *CatalogService) Update(
	ctx context.Context,
	request dto.UpdateListingRequest,
) (*dto.ListingResponse, error) {
	if !isValidUUID(request.ID) {
		return nil, fmt.Errorf("service update: %w", repository.ErrNotFound)
	}
	if err := validateUpdate(request); err != nil {
		return nil, fmt.Errorf("service update: %w", err)
	}

	listing := model.UpdateListing{
		ID:       request.ID,
		SellerID: request.SellerID,
		Title:    request.Title,
		Price:    request.Price,
		Stock:    request.Stock,
	}
	updated, err := s.repository.Update(ctx, listing)
	if err != nil {
		return nil, fmt.Errorf("service update: %w", err)
	}

	response := mapper.ToListingResponse(*updated)
	return &response, nil
}

func (s *CatalogService) Reserve(
	ctx context.Context,
	request dto.ReserveListingRequest,
) (*dto.ReserveListingResponse, error) {
	if err := validateReserve(request); err != nil {
		return nil, fmt.Errorf("service reserve: %w", err)
	}

	remaining, err := s.repository.Reserve(ctx, request.ListingID, request.Qty)
	if err != nil {
		return nil, fmt.Errorf("service reserve: %w", err)
	}

	return &dto.ReserveListingResponse{Remaining: remaining}, nil
}

func (s *CatalogService) ApplyOrderCreated(
	ctx context.Context,
	event dto.OrderCreatedEvent,
) (bool, error) {
	if err := validateOrderCreated(event); err != nil {
		return false, fmt.Errorf("service apply order created: %w", err)
	}

	applied, err := s.repository.ApplyOrderCreated(ctx, event.EventID, event.ListingID, event.Qty)
	if err != nil {
		return false, fmt.Errorf("service apply order created: %w", err)
	}

	return applied, nil
}

func (s *CatalogService) Delete(
	ctx context.Context,
	id,
	sellerID string,
) error {
	if err := s.repository.Delete(ctx, id, sellerID); err != nil {
		return fmt.Errorf("service delete: %w", err)
	}

	return nil
}
