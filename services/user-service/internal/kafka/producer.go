package kaf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

const (
	flushTimeout = 500
)

var (
	errUnknownType = errors.New("unknown event type")
)

type UserRegisteredEvent struct {
	UserID    int64    `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"` 
	Timestamp int64  `json:"timestamp"`
}

type Producer struct {
	producer *kafka.Producer
}

func NewProducer(address []string) (*Producer, error) {
	conf := &kafka.ConfigMap{
		"bootstrap.servers": strings.Join(address, ","),
	}
	p, err := kafka.NewProducer(conf)
	if err != nil {
		return nil, fmt.Errorf("error with new producer: %w", err)
	}
	return &Producer{producer: p}, nil
}

func (p *Producer) Produce(userID int64, email, name string) error {
	event := UserRegisteredEvent{
		UserID:    userID,
		Email:     email,
		Name:      name,
		Role:      "customer",
		Timestamp: time.Now().Unix(),
	}

	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshaling event: %w", err)
	}
	var topic string = "user.registered"
	kafkamsg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Value:     value,
		Key:       []byte(strconv.FormatInt(userID,10)),
		Timestamp: time.Now(),
	}

	kafkachan := make(chan kafka.Event)
	if err := p.producer.Produce(kafkamsg, kafkachan); err != nil {
		return fmt.Errorf("error while producing message: %w", err)
	}

	e := <-kafkachan
	switch ev := e.(type) {
	case *kafka.Message:
		return nil
	case kafka.Error:
		return ev
	default:
		return errUnknownType
	}
}

func (p *Producer) Close() {
	p.producer.Flush(flushTimeout)
	p.producer.Close()
}
