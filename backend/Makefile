APP_NAME=github.com/khiemnd777/legal_api
MIGRATIONS_DIR?=infra/migrations
MIGRATION_TABLE?=schema_migrations
MIGRATION_FILES?=\
	$(MIGRATIONS_DIR)/001_init.sql \
	$(MIGRATIONS_DIR)/002_phase1_legal_rag_config.sql \
	$(MIGRATIONS_DIR)/003_phase1_ai_config_safety.sql \
	$(MIGRATIONS_DIR)/004_phase2b_retrieval_config_management.sql \
	$(MIGRATIONS_DIR)/005_ai_answer_traces.sql \
	$(MIGRATIONS_DIR)/006_phase15_vector_repair.sql \
	$(MIGRATIONS_DIR)/007_auth_users.sql \
	$(MIGRATIONS_DIR)/008_force_password_change.sql \
	$(MIGRATIONS_DIR)/009_chat_conversations.sql

.PHONY: api worker run migrate migrate-baseline migrate-status docker-up docker-down

api:
	go build -o bin/api ./cmd/api

worker:
	go build -o bin/worker ./cmd/worker

run:
	go run ./cmd/api

migrate:
	@set -eu; \
	if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	POSTGRES_DSN="postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@$${POSTGRES_HOST}:$${POSTGRES_PORT}/$${POSTGRES_DB}?sslmode=$${POSTGRES_SSLMODE}"; \
	psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS $(MIGRATION_TABLE) (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());"; \
	for file in $(MIGRATION_FILES); do \
		version=$$(basename "$$file"); \
		applied=$$(psql "$$POSTGRES_DSN" -At -v ON_ERROR_STOP=1 -c "SELECT 1 FROM $(MIGRATION_TABLE) WHERE version = '$$version' LIMIT 1;"); \
		if [ "$$applied" = "1" ]; then \
			echo "Skipping $$version (already applied)"; \
			continue; \
		fi; \
		echo "Applying $$version"; \
		psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$$file"; \
		psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "INSERT INTO $(MIGRATION_TABLE) (version) VALUES ('$$version');"; \
	done

migrate-baseline:
	@set -eu; \
	if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	POSTGRES_DSN="postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@$${POSTGRES_HOST}:$${POSTGRES_PORT}/$${POSTGRES_DB}?sslmode=$${POSTGRES_SSLMODE}"; \
	psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS $(MIGRATION_TABLE) (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());"; \
	for file in $(MIGRATION_FILES); do \
		version=$$(basename "$$file"); \
		psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "INSERT INTO $(MIGRATION_TABLE) (version) VALUES ('$$version') ON CONFLICT (version) DO NOTHING;"; \
		echo "Baselined $$version"; \
	done

migrate-status:
	@set -eu; \
	if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	POSTGRES_DSN="postgres://$${POSTGRES_USER}:$${POSTGRES_PASSWORD}@$${POSTGRES_HOST}:$${POSTGRES_PORT}/$${POSTGRES_DB}?sslmode=$${POSTGRES_SSLMODE}"; \
	psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS $(MIGRATION_TABLE) (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());"; \
	psql "$$POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "SELECT version, applied_at FROM $(MIGRATION_TABLE) ORDER BY version;"

docker-up:
	docker compose --env-file .env -f docker/docker-compose.yml up --build

docker-down:
	docker compose --env-file .env -f docker/docker-compose.yml down
