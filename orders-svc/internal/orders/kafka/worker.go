package kafka

import (
	"context"
	"log"
	"marketplace/orders-svc/internal/orders/model"
	"time"
)

type MessagePublisher interface {
	SendUnpublishedEvents(ctx context.Context, limit int) ([]model.OutboxEvent, error)
}

type KafkaWorker struct {
	mp MessagePublisher
}

func NewWorker(mp MessagePublisher) *KafkaWorker {
	return &KafkaWorker{
		mp: mp,
	}
}

func (w *KafkaWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}

			ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*2)
			_, err := w.mp.SendUnpublishedEvents(ctxTimeout, 100)
			cancel()

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("WARN: outbox publication failed: %v", err)
			}
		}
	}
}
