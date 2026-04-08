.PHONY: build run lint test keys

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

lint:
	gofmt -l .
	go vet ./...
	golangci-lint run --timeout=5m

test:
	go test -race ./...

keys:
	mkdir -p .keys
	openssl genrsa -out .keys/private.pem 4096
	openssl rsa -in .keys/private.pem -pubout -out .keys/public.pem
	@echo "Keys generated in .keys/ (gitignored)"

migrate:
	psql "$(DATABASE_URL)" -f migrations/001_init.sql
