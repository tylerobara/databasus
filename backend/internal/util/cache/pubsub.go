package cache_utils

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/valkey-io/valkey-go"

	"databasus-backend/internal/util/logger"
)

type PubSubManager struct {
	client        valkey.Client
	subscriptions map[string]context.CancelFunc
	mu            sync.RWMutex
	logger        *slog.Logger
}

func NewPubSubManager() *PubSubManager {
	return &PubSubManager{
		client:        getCache(),
		subscriptions: make(map[string]context.CancelFunc),
		logger:        logger.GetLogger(),
	}
}

func (m *PubSubManager) Subscribe(
	ctx context.Context,
	channel string,
	handler func(message string),
) error {
	m.mu.Lock()
	if _, exists := m.subscriptions[channel]; exists {
		m.mu.Unlock()
		return fmt.Errorf("already subscribed to channel: %s", channel)
	}

	subCtx, cancel := context.WithCancel(ctx)
	m.subscriptions[channel] = cancel
	m.mu.Unlock()

	go m.subscriptionLoop(subCtx, channel, handler)

	return nil
}

func (m *PubSubManager) Publish(ctx context.Context, channel, message string) error {
	cmd := m.client.B().Publish().Channel(channel).Message(message).Build()
	result := m.client.Do(ctx, cmd)

	if err := result.Error(); err != nil {
		m.logger.Error("Failed to publish message to Redis", "channel", channel, "error", err)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (m *PubSubManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for channel, cancel := range m.subscriptions {
		cancel()
		delete(m.subscriptions, channel)
	}

	return nil
}

func (m *PubSubManager) subscriptionLoop(
	ctx context.Context,
	channel string,
	handler func(message string),
) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Panic in subscription handler", "channel", channel, "panic", r)
		}
	}()

	m.logger.Info("Starting subscription", "channel", channel)

	err := m.client.Receive(
		ctx,
		m.client.B().Subscribe().Channel(channel).Build(),
		func(msg valkey.PubSubMessage) {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("Panic in message handler", "channel", channel, "panic", r)
				}
			}()

			handler(msg.Message)
		},
	)

	if err != nil && ctx.Err() == nil {
		m.logger.Error("Subscription error", "channel", channel, "error", err)
	} else if ctx.Err() != nil {
		m.logger.Info("Subscription cancelled", "channel", channel)
	}

	m.mu.Lock()
	delete(m.subscriptions, channel)
	m.mu.Unlock()
}
