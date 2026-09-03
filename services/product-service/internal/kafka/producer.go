package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/kafka"
)

const flushTimeoutMilliseconds = 5_000

var errUnknownDeliveryEvent = errors.New("unknown Kafka delivery event")

type producerClient interface {
	Produce(message *confluent.Message, deliveryChan chan confluent.Event) error
	Flush(timeoutMs int) int
	Close()
}

type Producer struct {
	client    producerClient
	closeOnce sync.Once
	closeErr  error
}

func NewProducer(addresses []string) (*Producer, error) {
	if len(addresses) == 0 {
		return nil, fmt.Errorf("Kafka broker list is empty")
	}
	client, err := confluent.NewProducer(&confluent.ConfigMap{
		"bootstrap.servers": strings.Join(addresses, ","),
	})
	if err != nil {
		return nil, fmt.Errorf("create Kafka producer: %w", err)
	}
	return &Producer{client: client}, nil
}

func (p *Producer) Publish(ctx context.Context, topic string, key, value []byte) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("Kafka producer is not initialized")
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("Kafka topic is required")
	}

	delivery := make(chan confluent.Event, 1)
	message := &confluent.Message{
		TopicPartition: confluent.TopicPartition{
			Topic:     &topic,
			Partition: confluent.PartitionAny,
		},
		Key:       append([]byte(nil), key...),
		Value:     append([]byte(nil), value...),
		Timestamp: time.Now().UTC(),
	}
	if err := p.client.Produce(message, delivery); err != nil {
		return fmt.Errorf("enqueue Kafka message: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-delivery:
		switch delivered := event.(type) {
		case *confluent.Message:
			if delivered.TopicPartition.Error != nil {
				return fmt.Errorf("Kafka delivery failed: %w", delivered.TopicPartition.Error)
			}
			return nil
		case confluent.Error:
			return fmt.Errorf("Kafka delivery failed: %w", delivered)
		default:
			return errUnknownDeliveryEvent
		}
	}
}

func (p *Producer) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		remaining := p.client.Flush(flushTimeoutMilliseconds)
		if remaining > 0 {
			p.closeErr = fmt.Errorf("close Kafka producer: %d messages were not delivered", remaining)
		}
		p.client.Close()
	})
	return p.closeErr
}
