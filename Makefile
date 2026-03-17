.PHONY: proto build test lint run migrate-up migrate-down up down

proto:
	buf generate

lint:
	buf lint
	golangci-lint run ./...

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -v -count=1

run: build
	./bin/server

migrate-up:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" && exit 1)
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" && exit 1)
	migrate -path migrations -database "$(DATABASE_URL)" down 1

up:
	docker compose up --build -d

down:
	docker compose down -v
