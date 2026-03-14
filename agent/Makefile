.PHONY: run build test lint e2e e2e-clean

# Usage: make run ARGS="start --pg-host localhost"
run:
	go run cmd/main.go $(ARGS)

build:
	CGO_ENABLED=0 go build -ldflags "-X main.Version=$(VERSION)" -o databasus-agent ./cmd/main.go

test:
	go test -count=1 -failfast ./internal/...

lint:
	golangci-lint fmt ./cmd/... ./internal/... ./e2e/... && golangci-lint run ./cmd/... ./internal/... ./e2e/...

e2e:
	cd e2e && docker compose build
	cd e2e && docker compose run --rm e2e-agent-builder
	cd e2e && docker compose up -d e2e-postgres e2e-mock-server
	cd e2e && docker compose run --rm e2e-agent-runner
	cd e2e && docker compose run --rm e2e-agent-docker
	cd e2e && docker compose down -v

e2e-clean:
	cd e2e && docker compose down -v --rmi local
	rm -rf e2e/artifacts
