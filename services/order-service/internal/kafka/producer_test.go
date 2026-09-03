package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/stretchr/testify/require"
)

type producerClientStub struct {
	deliveryError error
	deliver       bool
	flushResult   int
	closed        bool
}

func (s *producerClientStub) Produce(message *confluent.Message, delivery chan confluent.Event) error {
	if s.deliver {
		delivery <- &confluent.Message{
			TopicPartition: confluent.TopicPartition{Error: s.deliveryError},
			Key:            message.Key,
			Value:          message.Value,
		}
	}
	return nil
}

func (s *producerClientStub) Flush(int) int {
	return s.flushResult
}

func (s *producerClientStub) Close() {
	s.closed = true
}

func TestProducerPublishWaitsForDelivery(t *testing.T) {
	producer := &Producer{client: &producerClientStub{deliver: true}}
	require.NoError(t, producer.Publish(context.Background(), "order.created", []byte("17"), []byte("payload")))
}

func TestProducerPublishReturnsDeliveryError(t *testing.T) {
	deliveryErr := errors.New("broker rejected message")
	producer := &Producer{client: &producerClientStub{deliver: true, deliveryError: deliveryErr}}
	err := producer.Publish(context.Background(), "order.created", nil, []byte("payload"))
	require.ErrorIs(t, err, deliveryErr)
}

func TestProducerPublishHonorsContext(t *testing.T) {
	producer := &Producer{client: &producerClientStub{deliver: false}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, producer.Publish(ctx, "order.created", nil, nil), context.DeadlineExceeded)
}

func TestProducerCloseReportsUndeliveredMessages(t *testing.T) {
	client := &producerClientStub{flushResult: 2}
	producer := &Producer{client: client}
	require.Error(t, producer.Close())
	require.True(t, client.closed)
}
