package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"marketplace/catalog-svc/internal/listings/dto"
	"time"

	"github.com/segmentio/kafka-go"
)

type MessageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
}

type OrderCreatedService interface {
	ApplyOrderCreated(context.Context, dto.OrderCreatedEvent) (bool, error)
}

type KafkaConsumer struct {
	reader  MessageReader
	service OrderCreatedService
}

func NewConsumer(reader MessageReader, service OrderCreatedService) *KafkaConsumer {
	return &KafkaConsumer{
		reader:  reader,
		service: service,
	}
}

func (c *KafkaConsumer) Start(ctx context.Context) {
	for ctx.Err() == nil {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.WarnContext(ctx, "Kafka fetch failed", "error", err)
			if !waitRetry(ctx) {
				return
			}
			continue
		}

		if !c.apply(ctx, message) || !c.commit(ctx, message) {
			return
		}
	}
}

func (c *KafkaConsumer) apply(ctx context.Context, message kafka.Message) bool {
	for {
		var event dto.OrderCreatedEvent
		err := json.Unmarshal(message.Value, &event)
		if err == nil {
			processCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			_, err = c.service.ApplyOrderCreated(processCtx, event)
			cancel()
		}
		if err == nil {
			return true
		}

		slog.WarnContext(ctx, "order.created processing failed",
			"partition", message.Partition, "offset", message.Offset, "error", err,
		)
		if !waitRetry(ctx) {
			return false
		}
	}
}

func (c *KafkaConsumer) commit(ctx context.Context, message kafka.Message) bool {
	for {
		commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		err := c.reader.CommitMessages(commitCtx, message)
		cancel()
		if err == nil {
			return true
		}

		slog.WarnContext(ctx, "Kafka commit failed",
			"partition", message.Partition, "offset", message.Offset, "error", err,
		)
		if !waitRetry(ctx) {
			return false
		}
	}
}

func waitRetry(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}
