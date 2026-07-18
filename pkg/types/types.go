// Package types defines the shared interfaces and configuration types used
// across the HotWire ecosystem.
package types

import "context"

// Config holds an arbitrary configuration payload as a raw JSON message.
type Config struct {
	Data []byte
}

// ConfigStore defines the interface for reading and writing configuration payloads
// in the backing key-value store.
type ConfigStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, val string) error
}

// ConfigPublisher defines the interface for publishing update signals to
// subscribers over a named channel.
type ConfigPublisher interface {
	Publish(ctx context.Context, channel, msg string) error
}
