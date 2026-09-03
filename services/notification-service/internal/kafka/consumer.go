package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	confluent "github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/events"
)

type MessageHandler interface {
	Handle(context.Context, []byte) error
}

type consumerClient interface {
	SubscribeTopics([]string, confluent.RebalanceCb) error
	ReadMessage(time.Duration) (*confluent.Message, error)
	CommitMessage(*confluent.Message) ([]confluent.TopicPartition, error)
	Close() error
}

type Config struct {
	Brokers         []string
	GroupID         string
	Topics          []string
	AutoOffsetReset string
	PollInterval    time.Duration
	HandlerTimeout  time.Duration
	MaxRetries      int
	RetryBaseDelay  time.Duration
	RetryMaxDelay   time.Duration
}

func (c Config) validate() error {
	if len(c.Brokers) == 0 || strings.TrimSpace(c.GroupID) == "" || len(c.Topics) == 0 {
		return fmt.Errorf("Kafka brokers, group id and topics are required")
	}
	if c.AutoOffsetReset != "earliest" && c.AutoOffsetReset != "latest" {
		return fmt.Errorf("Kafka auto offset reset must be earliest or latest")
	}
	if c.PollInterval <= 0 || c.HandlerTimeout <= 0 || c.MaxRetries <= 0 ||
		c.RetryBaseDelay <= 0 || c.RetryMaxDelay <= 0 || c.RetryBaseDelay > c.RetryMaxDelay {
		return fmt.Errorf("invalid Kafka consumer retry settings")
	}
	for _, topic := range c.Topics {
		if strings.TrimSpace(topic) == "" {
			return fmt.Errorf("Kafka topic cannot be empty")
		}
	}
	return nil
}

type Consumer struct {
	log     *slog.Logger
	client  consumerClient
	handler MessageHandler
	config  Config

	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
	errors    chan error
	running   bool
	closeOnce sync.Once
	closeErr  error
}

func New(log *slog.Logger, handler MessageHandler, config Config) (*Consumer, error) {
	if handler == nil {
		return nil, fmt.Errorf("notification event handler is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	client, err := confluent.NewConsumer(&confluent.ConfigMap{
		"bootstrap.servers":        strings.Join(config.Brokers, ","),
		"group.id":                 strings.TrimSpace(config.GroupID),
		"auto.offset.reset":        config.AutoOffsetReset,
		"enable.auto.commit":       false,
		"enable.auto.offset.store": false,
	})
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}
	return newWithClient(log, client, handler, config), nil
}

func newWithClient(log *slog.Logger, client consumerClient, handler MessageHandler, config Config) *Consumer {
	if log == nil {
		log = slog.Default()
	}
	return &Consumer{log: log, client: client, handler: handler, config: config}
}

func (c *Consumer) Start(parent context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return fmt.Errorf("Kafka consumer is already running")
	}
	if err := c.client.SubscribeTopics(c.config.Topics, nil); err != nil {
		return fmt.Errorf("subscribe Kafka topics: %w", err)
	}
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.errors = make(chan error, 1)
	c.running = true
	go c.run(ctx)
	return nil
}

func (c *Consumer) Errors() <-chan error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errors
}

func (c *Consumer) Stop(ctx context.Context) error {
	c.mu.Lock()
	running := c.running
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()
	if running {
		cancel()
		select {
		case <-done:
		case <-ctx.Done():
			return errors.Join(fmt.Errorf("stop Kafka consumer: %w", ctx.Err()), c.close())
		}
	}
	return c.close()
}

func (c *Consumer) close() error {
	c.closeOnce.Do(func() {
		if c.client != nil {
			c.closeErr = c.client.Close()
		}
	})
	return c.closeErr
}

func (c *Consumer) run(ctx context.Context) {
	err := c.consume(ctx)
	c.mu.Lock()
	c.running = false
	c.errors <- err
	close(c.errors)
	close(c.done)
	c.mu.Unlock()
}

func (c *Consumer) consume(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		message, err := c.client.ReadMessage(c.config.PollInterval)
		if err != nil {
			if isPollTimeout(err) {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			var kafkaErr confluent.Error
			if errors.As(err, &kafkaErr) && kafkaErr.IsFatal() {
				return fmt.Errorf("fatal Kafka consumer error: %w", err)
			}
			c.log.Warn("Kafka consumer poll failed", "error", err)
			if err := waitContext(ctx, c.config.RetryBaseDelay); err != nil {
				return nil
			}
			continue
		}
		if message == nil {
			continue
		}
		if err := c.process(ctx, message); err != nil {
			return err
		}
	}
}

func (c *Consumer) process(ctx context.Context, message *confluent.Message) error {
	var handlerErr error
	for attempt := 1; attempt <= c.config.MaxRetries; attempt++ {
		handlerCtx, cancel := context.WithTimeout(ctx, c.config.HandlerTimeout)
		handlerErr = c.handler.Handle(handlerCtx, message.Value)
		cancel()
		if handlerErr == nil {
			break
		}
		if events.IsPermanent(handlerErr) {
			c.log.Error(
				"discarding unsupported or malformed Kafka event",
				"error", handlerErr,
				"topic", message.TopicPartition.Topic,
				"partition", message.TopicPartition.Partition,
				"offset", message.TopicPartition.Offset,
			)
			break
		}
		if attempt == c.config.MaxRetries {
			return fmt.Errorf("notification handler failed after %d attempts; offset remains uncommitted: %w", attempt, handlerErr)
		}
		delay := retryDelay(c.config.RetryBaseDelay, c.config.RetryMaxDelay, attempt)
		c.log.Warn("notification persistence failed; retrying", "attempt", attempt, "delay", delay, "error", handlerErr)
		if err := waitContext(ctx, delay); err != nil {
			return nil
		}
	}
	if _, err := c.client.CommitMessage(message); err != nil {
		return fmt.Errorf("commit Kafka offset: %w", err)
	}
	return nil
}

func isPollTimeout(err error) bool {
	var kafkaErr confluent.Error
	return errors.As(err, &kafkaErr) && kafkaErr.Code() == confluent.ErrTimedOut
}

func retryDelay(base, maximum time.Duration, attempt int) time.Duration {
	delay := base
	for range attempt - 1 {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
