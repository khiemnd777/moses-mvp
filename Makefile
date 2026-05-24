LOCAL_COMPOSE := ./install/local-compose.sh

.DEFAULT_GOAL := help

.PHONY: help up down stop restart log ps migrate verify web-install web-add

help:
	@echo "Available root targets:"
	@echo "  make up       Build/start local Docker Compose stack"
	@echo "  make down     Stop/remove local Docker Compose stack"
	@echo "  make stop     Stop local Docker Compose stack"
	@echo "  make restart  Restart local Docker Compose stack"
	@echo "  make log      Tail local Compose logs"
	@echo "  make ps       Show local Compose services"
	@echo "  make migrate  Run local database migrations"
	@echo "  make verify   Smoke-test local Compose stack"
	@echo "  make web-install       Install frontend dependencies in the web container"
	@echo "  make web-add PKG=...   Add frontend package(s) through the web container"

up:
	$(LOCAL_COMPOSE) up

down:
	$(LOCAL_COMPOSE) down

stop:
	$(LOCAL_COMPOSE) stop

restart:
	$(LOCAL_COMPOSE) restart

log:
	$(LOCAL_COMPOSE) logs

ps:
	$(LOCAL_COMPOSE) ps

migrate:
	$(LOCAL_COMPOSE) migrate

verify:
	$(LOCAL_COMPOSE) verify

web-install:
	$(LOCAL_COMPOSE) web-install

web-add:
	@if [ -z "$(PKG)" ]; then echo "Usage: make web-add PKG=<package...>" >&2; exit 1; fi
	$(LOCAL_COMPOSE) web-add $(PKG)
