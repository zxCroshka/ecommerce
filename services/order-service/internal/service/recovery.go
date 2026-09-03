package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type RecoveryConfig struct {
	PollInterval time.Duration
	RecoveryAge  time.Duration
	OrderTimeout time.Duration
	BatchSize    int
}

type Recovery struct {
	log    *slog.Logger
	orders *OrderService
	config RecoveryConfig

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

func NewRecovery(log *slog.Logger, orders *OrderService, config RecoveryConfig) (*Recovery, error) {
	if orders == nil || config.PollInterval <= 0 || config.RecoveryAge <= 0 ||
		config.OrderTimeout <= 0 || config.BatchSize <= 0 {
		return nil, fmt.Errorf("invalid pending order recovery config")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Recovery{log: log, orders: orders, config: config}, nil
}

func (r *Recovery) Start(parent context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return fmt.Errorf("pending order recovery is already running")
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.done = make(chan struct{})
	r.running = true
	go r.run(ctx)
	return nil
}

func (r *Recovery) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop pending order recovery: %w", ctx.Err())
	}
}

func (r *Recovery) run(ctx context.Context) {
	defer func() {
		r.mu.Lock()
		r.running = false
		close(r.done)
		r.mu.Unlock()
	}()
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		if err := r.recoverBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.log.Error("pending order recovery iteration failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Recovery) recoverBatch(ctx context.Context) error {
	ids, err := r.orders.ListRecoverablePending(ctx, r.config.RecoveryAge, r.config.BatchSize)
	if err != nil {
		return err
	}
	for _, orderID := range ids {
		orderCtx, cancel := context.WithTimeout(ctx, r.config.OrderTimeout)
		err := r.orders.RecoverPending(orderCtx, orderID)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			r.log.Warn("pending order recovery failed", "order_id", orderID, "error", err)
		}
	}
	return nil
}
