# HotWire

[![CI](https://github.com/its-the-vibe/HotWire/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/its-the-vibe/HotWire/actions/workflows/ci.yaml)

A lightweight, high-performance, real-time configuration hot-reloading ecosystem in Go.  
HotWire uses a lock-free **Signal + Fetch** model via Redis to push configuration changes to every connected client instantly — no application restarts required.

---

## Architecture

| Component | Role |
|-----------|------|
| **Redis KV** | Stores the configuration payload |
| **Redis Pub/Sub** | Delivers fire-and-forget reload signals |
| **Writer (CLI)** | Validates, writes, and signals updates |
| **Reader (SDK)** | Embedded library that subscribes and swaps config atomically |

```
hotwire/
├── cmd/
│   └── hotwire/          # Cobra CLI – The Writer
├── pkg/
│   ├── reader/           # Client SDK – The Reader
│   │   ├── client.go     # Atomic .Get() + fetchAndStore
│   │   └── listener.go   # Background Pub/Sub loop
│   └── types/            # Shared interfaces & Config type
├── go.mod
└── README.md
```

---

## Development

Run the complete local verification suite:

```bash
make
```

Available targets:

| Target | Description |
|--------|-------------|
| `make build` | Build the CLI as `./hotwire` |
| `make fmt` | Verify Go formatting |
| `make lint` | Run `go vet` |
| `make test` | Run the test suite |
| `make test-race` | Run tests with the race detector |

---

## Quick Start

### Prerequisites

- Go 1.24+
- Redis 6+

### Install the CLI

```bash
go install github.com/its-the-vibe/HotWire/cmd/hotwire@latest
```

### Push a configuration update

```bash
# Write config.json to Redis and signal all clients to reload
hotwire config set --app my-service --file ./config.json
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--app` | *(required)* | Application name (key: `config:<app>`, channel: `ch:<app>`) |
| `--file` | *(required)* | Path to a valid JSON configuration file |
| `--redis-addr` | `localhost:6379` | Redis server address |
| `--redis-password` | *(empty)* | Redis password |
| `--redis-db` | `0` | Redis database index |

---

## Embedding the Reader SDK

```go
import (
    "context"
    "log"

    "github.com/its-the-vibe/HotWire/pkg/reader"
    "github.com/redis/go-redis/v9"
)

func main() {
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // New fetches the current config immediately (boot loop),
    // then launches a background subscriber.
    client, err := reader.New(ctx, rdb, "my-service")
    if err != nil {
        log.Fatal(err)
    }

    // Get is completely lock-free (atomic pointer load).
    cfg := client.Get()
    log.Printf("current config: %s", cfg.Data)
}
```

### Behaviour

1. **Boot loop** — on `New`, the SDK fetches the current value from `config:<app>` before returning.
2. **Reload** — on every `"reload"` message on `ch:<app>`, the SDK fetches and atomically swaps the in-memory pointer.
3. **Resiliency** — if the Pub/Sub connection drops, the background loop reconnects automatically and performs a fresh fetch to reconcile any updates missed during the outage.

---

## Running Tests

```bash
# Standard test suite
make test

# Race-enabled test suite
make test-race
```

The test suite covers:

- **Zero-race reads** — 100 goroutines calling `.Get()` concurrently while updates are published.
- **Reload signal** — verifies the in-memory config is swapped after a `"reload"` Pub/Sub message.
- **Network-drop reconciliation** — simulates a dropped connection and asserts the client fetches the latest config on reconnect.
