package reader_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/its-the-vibe/HotWire/pkg/reader"
	"github.com/redis/go-redis/v9"
)

// newTestClient is a helper that spins up a miniredis server, pre-seeds the
// given key, and returns a fully initialised *reader.Client together with its
// cancel function and the underlying miniredis instance.
func newTestClient(t *testing.T, appName, initialJSON string) (*reader.Client, *redis.Client, *miniredis.Miniredis, context.CancelFunc) {
	t.Helper()

	mr := miniredis.RunT(t)

	if initialJSON != "" {
		if err := mr.Set("config:"+appName, initialJSON); err != nil {
			t.Fatalf("miniredis Set: %v", err)
		}
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctx, cancel := context.WithCancel(context.Background())

	c, err := reader.New(ctx, rdb, appName)
	if err != nil {
		cancel()
		t.Fatalf("reader.New: %v", err)
	}

	return c, rdb, mr, cancel
}

// TestGet_InitialFetch verifies that New performs the mandatory boot fetch
// and that Get returns the pre-seeded configuration value immediately.
func TestGet_InitialFetch(t *testing.T) {
	c, _, _, cancel := newTestClient(t, "init", `{"ready":true}`)
	defer cancel()

	cfg := c.Get()
	if cfg == nil {
		t.Fatal("expected non-nil Config after boot fetch")
	}
	if string(cfg.Data) != `{"ready":true}` {
		t.Fatalf("unexpected config data: %s", cfg.Data)
	}
}

// TestGet_ZeroRaceConditions spawns 100 goroutines that all call Get()
// concurrently while a background loop publishes reload signals.  Run with
// `go test -race` to verify there are zero data races.
func TestGet_ZeroRaceConditions(t *testing.T) {
	const goroutines = 100

	c, rdb, mr, cancel := newTestClient(t, "race", `{"version":1}`)
	defer cancel()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Background updater: publish several reload signals while readers run.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 2; i <= 5; i++ {
			if err := mr.Set("config:race", `{"version":2}`); err != nil {
				return
			}
			_ = rdb.Publish(ctx, "ch:race", "reload")
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// 100 concurrent readers – every one must succeed without racing.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = c.Get() // must be data-race-free
				runtime_Gosched()
			}
		}()
	}

	wg.Wait()
}

// runtime_Gosched is a thin wrapper so the test file compiles without a
// direct import of the runtime package.
func runtime_Gosched() { time.Sleep(0) }

// TestReloadSignal verifies that a "reload" Pub/Sub message causes the client
// to fetch the updated configuration from Redis.
func TestReloadSignal(t *testing.T) {
	c, rdb, mr, cancel := newTestClient(t, "reload", `{"version":1}`)
	defer cancel()

	ctx := context.Background()

	// Update the value in Redis and publish the reload signal.
	if err := mr.Set("config:reload", `{"version":2}`); err != nil {
		t.Fatalf("miniredis Set: %v", err)
	}
	if err := rdb.Publish(ctx, "ch:reload", "reload").Err(); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Poll until the atomic swap is visible.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cfg := c.Get()
		if cfg != nil && string(cfg.Data) == `{"version":2}` {
			return // success
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("config was not updated after reload signal; got: %s", c.Get().Data)
}

// TestNetworkDrop_ReconciliationOnReconnect simulates a dropped Pub/Sub
// connection.  It verifies that upon reconnection the client forces a manual
// fetch and reconciles any updates that arrived during the outage.
func TestNetworkDrop_ReconciliationOnReconnect(t *testing.T) {
	c, _, mr, cancel := newTestClient(t, "droptest", `{"version":1}`)
	defer cancel()

	// Confirm the initial boot fetch succeeded.
	initial := c.Get()
	if initial == nil || string(initial.Data) != `{"version":1}` {
		t.Fatalf("unexpected initial config: %v", initial)
	}

	// Update Redis to v2 WITHOUT publishing a signal, then restart miniredis
	// to force all existing connections (including the Pub/Sub one) to drop.
	if err := mr.Set("config:droptest", `{"version":2}`); err != nil {
		t.Fatalf("miniredis Set v2: %v", err)
	}

	// Restart drops all client connections.  Miniredis comes back on the same
	// address; go-redis will reconnect automatically.  Our subscription loop
	// performs a reconcile fetch on every (re)connect, so it must pick up v2.
	mr.Restart()

	// Re-seed after restart (miniredis data does not persist across restarts).
	if err := mr.Set("config:droptest", `{"version":2}`); err != nil {
		t.Fatalf("miniredis Set v2 after restart: %v", err)
	}

	// Poll until the reconnect + reconcile fetch updates the in-memory config.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cfg := c.Get()
		if cfg != nil && string(cfg.Data) == `{"version":2}` {
			return // success
		}
		time.Sleep(50 * time.Millisecond)
	}

	got := "<nil>"
	if cfg := c.Get(); cfg != nil {
		got = string(cfg.Data)
	}
	t.Fatalf("config not reconciled after network drop; got: %s", got)
}
