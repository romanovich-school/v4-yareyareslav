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

const testListingID = "550e8400-e29b-41d4-a716-446655440000"
const testSellerID = "550e8400-e29b-41d4-a716-446655440001"

var testCreatedAt = time.Date(2026, 10, 12, 12, 0, 0, 123000000, time.FixedZone("MSK", 3*60*60))

func newCatalogTestPool(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	pool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer pool.Close()
		if err := pool.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})
	return pool
}

func listingRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "title", "price", "stock", "sold", "seller_id", "created_at"})
}

func TestCreate(t *testing.T) {
	failure := errors.New("insert failed")
	for _, wantErr := range []error{nil, failure} {
		name := "success"
		if wantErr != nil {
			name = "database failure"
		}
		t.Run(name, func(t *testing.T) {
			pool := newCatalogTestPool(t)
			input := model.Listing{Title: "milk", Price: 5000, Stock: 0, Sold: 99, SellerID: testSellerID}
			query := pool.ExpectQuery(`INSERT INTO listings \(id, title, price, stock, sold, seller_id\) VALUES \(gen_random_uuid\(\), \$1, \$2, \$3, \$4, \$5\) RETURNING id::text, created_at`).
				WithArgs(input.Title, input.Price, input.Stock, 0, input.SellerID)
			if wantErr != nil {
				query.WillReturnError(wantErr)
			} else {
				query.WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(testListingID, testCreatedAt))
			}
			got, err := NewPGRepository(pool).Create(context.Background(), input)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			if wantErr != nil {
				if got != nil {
					t.Fatal("returned listing on failure")
				}
				return
			}
			want := input
			want.ID, want.Sold, want.CreatedAt = testListingID, 0, testCreatedAt.UTC().Format(time.RFC3339Nano)
			if got == nil || *got != want {
				t.Fatalf("listing = %+v, want %+v", got, want)
			}
		})
	}
}

func TestGet(t *testing.T) {
	failure := errors.New("read failed")
	for _, tc := range []struct {
		name           string
		dbErr, wantErr error
	}{
		{name: "success"},
		{name: "missing", dbErr: pgx.ErrNoRows, wantErr: ErrNotFound},
		{name: "database failure", dbErr: failure, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := newCatalogTestPool(t)
			query := pool.ExpectQuery(`SELECT id::text, title, price, stock, sold, seller_id::text, created_at FROM listings WHERE id = \$1`).WithArgs(testListingID)
			if tc.dbErr != nil {
				query.WillReturnError(tc.dbErr)
			} else {
				query.WillReturnRows(listingRows().AddRow(testListingID, "milk", int64(5000), int64(5), int64(2), testSellerID, testCreatedAt))
			}
			got, err := NewPGRepository(pool).Get(context.Background(), testListingID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatal("returned listing on failure")
				}
				return
			}
			want := model.Listing{ID: testListingID, Title: "milk", Price: 5000, Stock: 5, Sold: 2, SellerID: testSellerID, CreatedAt: testCreatedAt.UTC().Format(time.RFC3339Nano)}
			if got == nil || *got != want {
				t.Fatalf("listing = %+v, want %+v", got, want)
			}
		})
	}
}

func TestList(t *testing.T) {
	failure := errors.New("database failure")
	for _, tc := range []struct {
		name    string
		step    string
		wantLen int
		wantErr error
	}{
		{name: "page", wantLen: 1},
		{name: "empty page", step: "empty"},
		{name: "query failure", step: "query", wantErr: failure},
		{name: "scan failure", step: "scan"},
		{name: "iteration failure", step: "rows", wantErr: failure},
		{name: "count failure", step: "count", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := newCatalogTestPool(t)
			query := pool.ExpectQuery(`SELECT id::text, title, price, stock, sold, seller_id::text, created_at FROM listings ORDER BY created_at ASC, id ASC LIMIT \$1 OFFSET \$2`).WithArgs(20, 5)
			rows := listingRows()
			switch tc.step {
			case "query":
				query.WillReturnError(failure)
			case "scan":
				query.WillReturnRows(rows.AddRow(testListingID, "milk", "invalid price", int64(5), int64(2), testSellerID, testCreatedAt))
			case "rows":
				query.WillReturnRows(rows.AddRow(testListingID, "milk", int64(5000), int64(5), int64(2), testSellerID, testCreatedAt).RowError(0, failure))
			default:
				if tc.step != "empty" {
					rows.AddRow(testListingID, "milk", int64(5000), int64(5), int64(2), testSellerID, testCreatedAt)
				}
				query.WillReturnRows(rows).RowsWillBeClosed()
				count := pool.ExpectQuery(`SELECT COUNT\(\*\) FROM listings`)
				if tc.step == "count" {
					count.WillReturnError(failure)
				} else {
					count.WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(42)))
				}
			}
			got, total, err := NewPGRepository(pool).List(context.Background(), 20, 5)
			if tc.step == "scan" {
				if err == nil {
					t.Fatal("expected scan error")
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			if got == nil || len(got) != tc.wantLen || total != 42 {
				t.Fatalf("page = %+v, total = %d", got, total)
			}
			if len(got) > 0 && got[0].CreatedAt != testCreatedAt.UTC().Format(time.RFC3339Nano) {
				t.Fatalf("unexpected timestamp: %s", got[0].CreatedAt)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	failure := errors.New("database failure")
	for _, tc := range []struct {
		name, step string
		wantErr    error
	}{
		{name: "success"},
		{name: "missing", step: "missing", wantErr: ErrNotFound},
		{name: "another seller", step: "forbidden", wantErr: ErrForbidden},
		{name: "begin failure", step: "begin", wantErr: failure},
		{name: "read failure", step: "read", wantErr: failure},
		{name: "delete failure", step: "delete", wantErr: failure},
		{name: "no rows deleted", step: "zero", wantErr: ErrNotFound},
		{name: "commit failure", step: "commit", wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := newCatalogTestPool(t)
			begin := pool.ExpectBegin()
			if tc.step == "begin" {
				begin.WillReturnError(failure)
			} else {
				lookup := pool.ExpectQuery(`SELECT seller_id = \$2::uuid FROM listings WHERE id = \$1 FOR UPDATE`).WithArgs(testListingID, testSellerID)
				switch tc.step {
				case "missing":
					lookup.WillReturnError(pgx.ErrNoRows)
				case "read":
					lookup.WillReturnError(failure)
				case "forbidden":
					lookup.WillReturnRows(pgxmock.NewRows([]string{"owner"}).AddRow(false))
				default:
					lookup.WillReturnRows(pgxmock.NewRows([]string{"owner"}).AddRow(true))
					query := pool.ExpectExec(`DELETE FROM listings WHERE id = \$1 AND seller_id = \$2`).WithArgs(testListingID, testSellerID)
					switch tc.step {
					case "delete":
						query.WillReturnError(failure)
					case "zero":
						query.WillReturnResult(pgxmock.NewResult("DELETE", 0))
					default:
						query.WillReturnResult(pgxmock.NewResult("DELETE", 1))
						commit := pool.ExpectCommit()
						if tc.step == "commit" {
							commit.WillReturnError(failure)
						}
					}
				}
				pool.ExpectRollback()
			}
			if err := NewPGRepository(pool).Delete(context.Background(), testListingID, testSellerID); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
