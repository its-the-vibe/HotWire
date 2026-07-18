package reader

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// subscribe runs a persistent, self-healing Pub/Sub loop for the given channel.
// On every (re)connection it performs an explicit fetch so that any updates
// missed during a network interruption are reconciled immediately.  The loop
// exits only when ctx is cancelled.
func (c *Client) subscribe(ctx context.Context, channel string) {
	for {
		if ctx.Err() != nil {
			return
		}

		sub := c.rdb.Subscribe(ctx, channel)

		// Reconcile any updates that arrived while the connection was down.
		if err := c.fetchAndStore(ctx); err != nil {
			log.Printf("hotwire: reconcile fetch on (re)connect failed: %v", err)
		}

		dropped := c.drainMessages(ctx, sub)

		_ = sub.Close()

		if !dropped {
			// Context was cancelled – clean shutdown.
			return
		}

		log.Printf("hotwire: subscription to %q dropped, reconnecting…", channel)
	}
}

// drainMessages reads messages from the Pub/Sub subscription until the
// connection drops or the context is cancelled.  It returns true when the
// message channel was closed unexpectedly (network drop) and false when the
// context was cancelled (clean shutdown).
func (c *Client) drainMessages(ctx context.Context, sub *redis.PubSub) bool {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return false
		case msg, ok := <-ch:
			if !ok {
				// Channel closed – connection dropped.
				return true
			}
			if msg.Payload == "reload" {
				if err := c.fetchAndStore(ctx); err != nil {
					log.Printf("hotwire: fetch on reload signal failed: %v", err)
				}
			}
		}
	}
}
