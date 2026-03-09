APP_NAME=github.com/khiemnd777/legal_api
CONFIG_PATH?=config/config.yaml
POSTGRES_DSN?=postgres://legal:legal@localhost:5433/legal_rag?sslmode=disable

.PHONY: api worker run migrate docker-up docker-down

api:
	go build -o bin/api ./cmd/api

worker:
	go build -o bin/worker ./cmd/worker

run:
	CONFIG_PATH=$(CONFIG_PATH) go run ./cmd/api

migrate:
	psql $(POSTGRES_DSN) -f infra/migrations/001_init.sql

docker-up:
	docker compose -f docker/docker-compose.yml up --build

docker-down:
	docker compose -f docker/docker-compose.yml down
