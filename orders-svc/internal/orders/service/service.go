package service

import (
	"context"
	"errors"
	"fmt"
	"marketplace/orders-svc/internal/orders/dto"
	"marketplace/orders-svc/internal/orders/model"
	"time"
)

type Repository interface {
	Create(ctx context.Context, order model.Order, idempKey string) (*model.Order, error)
	ReserveIdempotencyKey(ctx context.Context, buyerID, key string) (bool, error)
	GetIdempotencyKeyState(ctx context.Context, buyerID, key string) (*model.IdempotencyKeyState, error)
	GetByID(ctx context.Context, orderID, buyerID string) (*model.Order, error)
	List(ctx context.Context, buyerID string, limit, offset int) ([]model.Order, int64, error)
}

type Catalog interface {
	Reserve(ctx context.Context, listingID string, qty int) error
}

var ErrInvalidLimit = errors.New("limit out of range between 0 and 100")
var ErrInvalidOffset = errors.New("offset must be a positive number")

type OrdersService struct {
	rep     Repository
	catalog Catalog
}

func NewOrdersService(rep Repository, catalog Catalog) *OrdersService {
	return &OrdersService{
		rep:     rep,
		catalog: catalog,
	}
}

func (s *OrdersService) Create(
	ctx context.Context,
	createDto dto.CreateOrderRequest,
	buyerID string,
	idempKey string,
) (*dto.CreateOrderResponse, error) {
	if idempKey != "" {
		res, err := s.rep.ReserveIdempotencyKey(ctx, buyerID, idempKey)
		if err != nil {
			return nil, fmt.Errorf("create order: %w", err)
		}

		if !res {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				if err := ctx.Err(); err != nil {
					return nil, fmt.Errorf("create order: %w", err)
				}

				stateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				state, err := s.rep.GetIdempotencyKeyState(stateCtx, buyerID, idempKey)
				cancel()
				if err != nil {
					return nil, fmt.Errorf("create order: %w", err)
				}

				if state != nil && state.OrderID != nil {
					order, err := s.rep.GetByID(ctx, *state.OrderID, buyerID)
					if err != nil {
						return nil, fmt.Errorf("create order: %w", err)
					}
					return &dto.CreateOrderResponse{
						ID:        order.ID,
						BuyerID:   order.BuyerID,
						ListingID: order.ListingID,
						Qty:       order.Qty,
						CreatedAt: order.CreatedAt,
					}, nil
				}

				if state == nil {
					reserved, err := s.rep.ReserveIdempotencyKey(ctx, buyerID, idempKey)
					if err != nil {
						return nil, fmt.Errorf("create order: %w", err)
					}
					if reserved {
						break
					}
				}

				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("create order: %w", ctx.Err())
				case <-ticker.C:
				}
			}
		}
	}

	reserveCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	err := s.catalog.Reserve(reserveCtx, createDto.ListingID, createDto.Qty)
	cancel()

	if err != nil {
		return nil, fmt.Errorf("reserve listing: %w", err)
	}

	createCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	order := model.Order{
		BuyerID:   buyerID,
		ListingID: createDto.ListingID,
		Qty:       createDto.Qty,
	}

	createdOrder, err := s.rep.Create(createCtx, order, idempKey)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	return &dto.CreateOrderResponse{
		ID:        createdOrder.ID,
		BuyerID:   createdOrder.BuyerID,
		ListingID: createdOrder.ListingID,
		Qty:       createdOrder.Qty,
		CreatedAt: createdOrder.CreatedAt,
	}, nil
}

func (s *OrdersService) List(
	ctx context.Context,
	buyerID string,
	limit,
	offset int,
) (*dto.ListResponse, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("list orders: %w", ErrInvalidLimit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("list orders: %w", ErrInvalidOffset)
	}

	orders, totalCount, err := s.rep.List(ctx, buyerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	listedOrders := make([]dto.OrderResponse, 0, len(orders))
	for _, order := range orders {
		listed := dto.OrderResponse{
			ID:        order.ID,
			BuyerID:   order.BuyerID,
			ListingID: order.ListingID,
			Qty:       order.Qty,
			CreatedAt: order.CreatedAt,
		}
		listedOrders = append(listedOrders, listed)
	}

	return &dto.ListResponse{
		Orders: listedOrders,
		Total:  totalCount,
	}, nil
}
