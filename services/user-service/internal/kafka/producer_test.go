package kaf

import (
	"context"
	"errors"
	"testing"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/stretchr/testify/require"
)

type fakeProducerClient struct {
	produce func(*confluent.Message, chan confluent.Event) error
	remain  int
	closed  bool
}

func (f *fakeProducerClient) Produce(message *confluent.Message, delivery chan confluent.Event) error {
	return f.produce(message, delivery)
}
func (f *fakeProducerClient) Flush(int) int { return f.remain }
func (f *fakeProducerClient) Close()        { f.closed = true }

func TestProducerWaitsForSuccessfulDelivery(t *testing.T) {
	client := &fakeProducerClient{produce: func(message *confluent.Message, delivery chan confluent.Event) error {
		require.Equal(t, "topic", *message.TopicPartition.Topic)
		delivery <- &confluent.Message{TopicPartition: confluent.TopicPartition{Partition: 0}}
		return nil
	}}
	producer := &Producer{client: client}
	require.NoError(t, producer.Publish(context.Background(), "topic", []byte("key"), []byte("value")))
}

func TestProducerReturnsTopicPartitionDeliveryError(t *testing.T) {
	deliveryErr := errors.New("broker rejected message")
	client := &fakeProducerClient{produce: func(_ *confluent.Message, delivery chan confluent.Event) error {
		delivery <- &confluent.Message{TopicPartition: confluent.TopicPartition{Error: deliveryErr}}
		return nil
	}}
	producer := &Producer{client: client}
	err := producer.Publish(context.Background(), "topic", nil, []byte("value"))
	require.ErrorIs(t, err, deliveryErr)
}

func TestProducerHonorsContextWhileWaitingForDelivery(t *testing.T) {
	client := &fakeProducerClient{produce: func(_ *confluent.Message, _ chan confluent.Event) error { return nil }}
	producer := &Producer{client: client}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, producer.Publish(ctx, "topic", nil, nil), context.DeadlineExceeded)
}

func TestProducerCloseReportsUndeliveredMessages(t *testing.T) {
	client := &fakeProducerClient{remain: 2, produce: func(*confluent.Message, chan confluent.Event) error { return nil }}
	producer := &Producer{client: client}
	require.Error(t, producer.Close())
	require.True(t, client.closed)
}
