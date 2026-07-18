// Package reader provides a high-performance, concurrent-safe client SDK that
// maintains an in-memory configuration by combining an atomic pointer for
// lock-free reads with a background Redis Pub/Sub subscription loop.
package reader

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/its-the-vibe/HotWire/pkg/types"
	"github.com/redis/go-redis/v9"
)

// Client subscribes to a Redis channel for an application and exposes the
// latest configuration via a completely lock-free Get call.
type Client struct {
	rdb    *redis.Client
	appKey string
	ptr    atomic.Pointer[types.Config]
}

// New creates a Client for the given appName.  It follows the mandatory boot
// sequence:
//
//  1. Fetch the current configuration from the Redis key-value store so the
//     caller has valid state immediately.
//  2. Launch a background goroutine that subscribes to the Pub/Sub channel and
//     keeps the in-memory pointer up-to-date.
//
// The provided context governs the lifetime of the subscription loop; cancel
// it to shut the client down gracefully.
func New(ctx context.Context, rdb *redis.Client, appName string) (*Client, error) {
	c := &Client{
		rdb:    rdb,
		appKey: fmt.Sprintf("config:%s", appName),
	}

	if err := c.fetchAndStore(ctx); err != nil {
		return nil, fmt.Errorf("hotwire: initial fetch for %q: %w", appName, err)
	}

	go c.subscribe(ctx, fmt.Sprintf("ch:%s", appName))

	return c, nil
}

// Get returns the most recently cached configuration.  The call is completely
// lock-free thanks to the underlying atomic pointer.
func (c *Client) Get() *types.Config {
	return c.ptr.Load()
}

// fetchAndStore retrieves the current configuration payload from Redis and
// atomically swaps the in-memory pointer.  If the key does not yet exist the
// in-memory pointer is left unchanged.
func (c *Client) fetchAndStore(ctx context.Context) error {
	val, err := c.rdb.Get(ctx, c.appKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key not yet populated; leave the current pointer unchanged.
			return nil
		}
		return err
	}
	c.ptr.Store(&types.Config{Data: []byte(val)})
	return nil
}
