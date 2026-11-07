BINARY := thermostat

.PHONY: tidy run-dev build test test-all

tidy:
	go mod tidy

run-dev:
	CGO_ENABLED=0 go run ./cmd/thermostat -dev

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/thermostat

# Runs the standard-library-only tests (no GUI/CGO required).
test:
	go test ./internal/fb/...

# Runs the full test suite (requires Ebiten and its runtime libraries).
test-all:
	go test ./...
