.PHONY: all build fmt lint test test-race

all: fmt lint test-race build

build:
	go build ./...

fmt:
	test -z "$$(gofmt -l .)"

lint:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...
