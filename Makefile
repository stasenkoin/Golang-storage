.PHONY: run build test tidy docker-up docker-down migrate-up migrate-down

run:
	go run ./cmd/file-storage

build:
	go build -o bin/file-storage ./cmd/file-storage

test:
	go test ./...

tidy:
	go mod tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	docker compose exec -T postgres psql -U postgres -d file_storage < migrations/001_init.sql

migrate-down:
	docker compose exec -T postgres psql -U postgres -d file_storage -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
