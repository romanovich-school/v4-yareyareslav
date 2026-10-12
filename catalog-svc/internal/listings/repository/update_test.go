package repository

import (
	"context"
	"errors"
	"marketplace/catalog-svc/internal/listings/model"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func TestUpdate(t *testing.T) {
	title := "updated"
	zero := int64(0)
	commitErr := errors.New("commit failed")
	createdAt := time.Date(2026, 10, 12, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

	for _, tc := range []struct {
		name      string
		patch     model.UpdateListing
		owner     bool
		lookupErr error
		commitErr error
		wantErr   error
		wantTitle string
		wantStock int64
	}{
		{name: "title only", patch: model.UpdateListing{Title: &title}, owner: true, wantTitle: title, wantStock: 5},
		{name: "zero stock", patch: model.UpdateListing{Stock: &zero}, owner: true, wantTitle: "original", wantStock: 0},
		{name: "missing", lookupErr: pgx.ErrNoRows, wantErr: ErrNotFound},
		{name: "another seller", wantErr: ErrForbidden},
		{name: "commit failure", owner: true, commitErr: commitErr, wantErr: commitErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			tc.patch.ID = "550e8400-e29b-41d4-a716-446655440000"
			tc.patch.SellerID = "550e8400-e29b-41d4-a716-446655440001"
			pool.ExpectBegin()
			lookup := pool.ExpectQuery(`SELECT seller_id = \$2::uuid FROM listings WHERE id = \$1 FOR UPDATE`).
				WithArgs(tc.patch.ID, tc.patch.SellerID)
			if tc.lookupErr != nil {
				lookup.WillReturnError(tc.lookupErr)
			} else {
				lookup.WillReturnRows(pgxmock.NewRows([]string{"is_owner"}).AddRow(tc.owner))
			}
			if tc.owner && tc.lookupErr == nil {
				pool.ExpectQuery(`UPDATE listings SET title = COALESCE\(\$2::text, title\), price = COALESCE\(\$3::bigint, price\), stock = COALESCE\(\$4::bigint, stock\) WHERE id = \$1 RETURNING id::text, title, price, stock, sold, seller_id::text, created_at`).
					WithArgs(tc.patch.ID, tc.patch.Title, tc.patch.Price, tc.patch.Stock).
					WillReturnRows(pgxmock.NewRows([]string{"id", "title", "price", "stock", "sold", "seller_id", "created_at"}).
						AddRow(tc.patch.ID, tc.wantTitle, int64(100), tc.wantStock, int64(2), tc.patch.SellerID, createdAt))
				commit := pool.ExpectCommit()
				if tc.commitErr != nil {
					commit.WillReturnError(tc.commitErr)
				}
			}
			pool.ExpectRollback()

			got, err := NewPGRepository(pool).Update(context.Background(), tc.patch)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("returned listing on failure")
				}
			} else if got == nil || got.Title != tc.wantTitle || got.Stock != tc.wantStock || got.Sold != 2 || got.Price != 100 || got.CreatedAt != createdAt.UTC().Format(time.RFC3339Nano) {
				t.Fatalf("unexpected listing: %+v", got)
			}
			if err := pool.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
