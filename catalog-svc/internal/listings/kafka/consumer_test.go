package kafka

import (
	"context"
	"errors"
	"marketplace/catalog-svc/internal/listings/dto"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/segmentio/kafka-go"
)

type readerStub struct {
	fetch  func(context.Context) (kafka.Message, error)
	commit func(context.Context, ...kafka.Message) error
}

func (r readerStub) FetchMessage(ctx context.Context) (kafka.Message, error) {
	return r.fetch(ctx)
}

func (r readerStub) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	return r.commit(ctx, messages...)
}

type serviceFunc func(context.Context, dto.OrderCreatedEvent) (bool, error)

func (f serviceFunc) ApplyOrderCreated(ctx context.Context, event dto.OrderCreatedEvent) (bool, error) {
	return f(ctx, event)
}

func TestConsumerRetriesBeforeFetchingNext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		message := kafka.Message{Topic: "order.created", Partition: 1, Offset: 12,
			Value: []byte(`{"event_id":"event","order_id":"order","buyer_id":"buyer","listing_id":"listing","qty":2,"created_at":"2026-10-13T09:00:00Z"}`),
		}
		var calls []string
		applies, commits := 0, 0
		reader := readerStub{
			fetch: func(context.Context) (kafka.Message, error) {
				calls = append(calls, "fetch")
				if len(calls) != 1 {
					t.Fatal("fetched next message before successful commit")
				}
				return message, nil
			},
			commit: func(commitCtx context.Context, messages ...kafka.Message) error {
				calls = append(calls, "commit")
				commits++
				if commitCtx.Err() != nil || len(messages) != 1 || !reflect.DeepEqual(messages[0], message) {
					t.Fatal("unexpected commit arguments")
				}
				if commits == 1 {
					return errors.New("broker unavailable")
				}
				cancel()
				return nil
			},
		}
		service := serviceFunc(func(processCtx context.Context, event dto.OrderCreatedEvent) (bool, error) {
			calls = append(calls, "apply")
			applies++
			deadline, ok := processCtx.Deadline()
			if !ok || time.Until(deadline) != 2*time.Second {
				t.Fatal("missing processing timeout")
			}
			want := dto.OrderCreatedEvent{EventID: "event", OrderID: "order", BuyerID: "buyer", ListingID: "listing", Qty: 2, CreatedAt: "2026-10-13T09:00:00Z"}
			if event != want {
				t.Fatalf("event = %+v, want %+v", event, want)
			}
			if applies == 1 {
				return false, errors.New("database unavailable")
			}
			return false, nil
		})
		NewConsumer(reader, service).Start(ctx)
		want := []string{"fetch", "apply", "apply", "commit", "commit"}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	})
}

func TestConsumerFinishesMessageOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	committed := false
	reader := readerStub{
		fetch: func(context.Context) (kafka.Message, error) {
			return kafka.Message{Value: []byte(`{"event_id":"event"}`)}, nil
		},
		commit: func(commitCtx context.Context, messages ...kafka.Message) error {
			if commitCtx.Err() != nil {
				t.Fatalf("commit canceled during shutdown: %v", commitCtx.Err())
			}
			committed = true
			return nil
		},
	}
	service := serviceFunc(func(processCtx context.Context, event dto.OrderCreatedEvent) (bool, error) {
		cancel()
		if processCtx.Err() != nil {
			t.Fatal("in-flight processing canceled")
		}
		return true, nil
	})
	NewConsumer(reader, service).Start(ctx)
	if !committed {
		t.Fatal("processed message not committed on shutdown")
	}
}

func TestConsumerDoesNotCommitInvalidMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reader := readerStub{
			fetch: func(context.Context) (kafka.Message, error) {
				return kafka.Message{Value: []byte(`{"qty":`)}, nil
			},
			commit: func(context.Context, ...kafka.Message) error {
				t.Fatal("invalid message committed")
				return nil
			},
		}
		service := serviceFunc(func(context.Context, dto.OrderCreatedEvent) (bool, error) {
			t.Fatal("service called for invalid JSON")
			return false, nil
		})
		done := make(chan struct{})
		go func() {
			defer close(done)
			NewConsumer(reader, service).Start(ctx)
		}()
		synctest.Wait()
		cancel()
		<-done
	})
}

func TestConsumerRecoversAfterFetchFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fetches := 0
		reader := readerStub{
			fetch: func(context.Context) (kafka.Message, error) {
				fetches++
				if fetches == 1 {
					return kafka.Message{}, errors.New("broker unavailable")
				}
				return kafka.Message{Value: []byte(`{}`)}, nil
			},
			commit: func(context.Context, ...kafka.Message) error {
				cancel()
				return nil
			},
		}
		service := serviceFunc(func(context.Context, dto.OrderCreatedEvent) (bool, error) { return true, nil })
		NewConsumer(reader, service).Start(ctx)
		if fetches != 2 {
			t.Fatalf("fetches = %d, want 2", fetches)
		}
	})
}
