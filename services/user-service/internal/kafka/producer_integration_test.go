//go:build integration


package kaf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Интеграционные тесты требуют запущенную Kafka
// Запуск: go test -tags=integration ./kafka/...

func TestProducer_Integration_Produce(t *testing.T) {
	// Проверяем, что Kafka доступна
	producer, err := NewProducer([]string{"localhost:9092"})
	if err != nil {
		t.Skip("Kafka not available, skipping integration test")
	}
	defer producer.Close()

	tests := []struct {
		nameTest string
		userID   int64
		email    string
		name     string
	}{
		{
			nameTest: "produce customer registration",
			userID:   1001,
			email:    "integration@example.com",
			name:     "Integration User",
		},
		{
			nameTest: "produce with empty name",
			userID:   1002,
			email:    "empty@example.com",
			name:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := producer.Produce(tt.userID, tt.email, tt.name)
			assert.NoError(t, err)
		})
	}
}

func TestProducer_Integration_Close(t *testing.T) {
	producer, err := NewProducer([]string{"localhost:9092"})
	if err != nil {
		t.Skip("Kafka not available, skipping integration test")
	}

	// Close не должен паниковать
	assert.NotPanics(t, func() {
		producer.Close()
	})
}
