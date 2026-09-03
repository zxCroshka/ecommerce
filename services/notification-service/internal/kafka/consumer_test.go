package kafka

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/events"
)

type consumerClientStub struct {
	mu          sync.Mutex
	messages    []*confluent.Message
	readErrors  []error
	commits     int
	closed      bool
	subscribed  []string
	commitError error
}

func (s *consumerClientStub) SubscribeTopics(topics []string, _ confluent.RebalanceCb) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribed = append([]string(nil), topics...)
	return nil
}

func (s *consumerClientStub) ReadMessage(time.Duration) (*confluent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.readErrors) > 0 {
		err := s.readErrors[0]
		s.readErrors = s.readErrors[1:]
		return nil, err
	}
	if len(s.messages) > 0 {
		message := s.messages[0]
		s.messages = s.messages[1:]
		return message, nil
	}
	return nil, confluent.NewError(confluent.ErrTimedOut, "poll timeout", false)
}

func (s *consumerClientStub) CommitMessage(*confluent.Message) ([]confluent.TopicPartition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.commitError != nil {
		return nil, s.commitError
	}
	s.commits++
	return nil, nil
}

func (s *consumerClientStub) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type messageHandlerStub struct {
	mu     sync.Mutex
	errors []error
	calls  int
}

func (s *messageHandlerStub) Handle(context.Context, []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if len(s.errors) == 0 {
		return nil
	}
	err := s.errors[0]
	s.errors = s.errors[1:]
	return err
}

func consumerConfig() Config {
	return Config{
		Brokers: []string{"broker:9092"}, GroupID: "notification-test", Topics: []string{"user.registered", "order.created"},
		AutoOffsetReset: "earliest", PollInterval: time.Millisecond, HandlerTimeout: time.Second,
		MaxRetries: 3, RetryBaseDelay: time.Millisecond, RetryMaxDelay: 2 * time.Millisecond,
	}
}

func testConsumer(client consumerClient, handler MessageHandler) *Consumer {
	return newWithClient(slog.New(slog.NewTextHandler(io.Discard, nil)), client, handler, consumerConfig())
}

func kafkaMessage() *confluent.Message {
	topic := "order.created"
	return &confluent.Message{TopicPartition: confluent.TopicPartition{Topic: &topic, Partition: 0, Offset: 12}, Value: []byte("event")}
}

func TestConsumerCommitsOnlyAfterSuccessfulHandling(t *testing.T) {
	client := &consumerClientStub{}
	handler := &messageHandlerStub{}
	require.NoError(t, testConsumer(client, handler).process(context.Background(), kafkaMessage()))
	require.Equal(t, 1, handler.calls)
	require.Equal(t, 1, client.commits)
}

func TestConsumerRetriesTemporaryFailureThenCommits(t *testing.T) {
	client := &consumerClientStub{}
	handler := &messageHandlerStub{errors: []error{errors.New("database unavailable"), nil}}
	require.NoError(t, testConsumer(client, handler).process(context.Background(), kafkaMessage()))
	require.Equal(t, 2, handler.calls)
	require.Equal(t, 1, client.commits)
}

func TestConsumerDoesNotCommitAfterRetryExhaustion(t *testing.T) {
	client := &consumerClientStub{}
	handler := &messageHandlerStub{errors: []error{
		errors.New("database unavailable"), errors.New("database unavailable"), errors.New("database unavailable"),
	}}
	err := testConsumer(client, handler).process(context.Background(), kafkaMessage())
	require.Error(t, err)
	require.Equal(t, 3, handler.calls)
	require.Zero(t, client.commits)
}

func TestConsumerCommitsRejectedPermanentEvent(t *testing.T) {
	client := &consumerClientStub{}
	handler := &messageHandlerStub{errors: []error{&events.PermanentError{Err: errors.New("unsupported version")}}}
	require.NoError(t, testConsumer(client, handler).process(context.Background(), kafkaMessage()))
	require.Equal(t, 1, handler.calls)
	require.Equal(t, 1, client.commits)
}

func TestConsumerReturnsCommitFailureForRedelivery(t *testing.T) {
	client := &consumerClientStub{commitError: errors.New("coordinator unavailable")}
	handler := &messageHandlerStub{}
	err := testConsumer(client, handler).process(context.Background(), kafkaMessage())
	require.ErrorContains(t, err, "commit Kafka offset")
	require.Equal(t, 1, handler.calls, "durable effect may already exist and must be deduplicated on redelivery")
	require.Zero(t, client.commits)
}

func TestConsumerGracefulShutdownClosesClient(t *testing.T) {
	client := &consumerClientStub{}
	consumer := testConsumer(client, &messageHandlerStub{})
	require.NoError(t, consumer.Start(context.Background()))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, consumer.Stop(ctx))
	client.mu.Lock()
	defer client.mu.Unlock()
	require.True(t, client.closed)
	require.Equal(t, []string{"user.registered", "order.created"}, client.subscribed)
}
