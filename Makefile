LOCAL_COMPOSE := ./install/local-compose.sh

.DEFAULT_GOAL := help

.PHONY: help up down stop restart log

help:
	@echo "Available root targets:"
	@echo "  make up       Build/start local Docker Compose stack"
	@echo "  make down     Stop/remove local Docker Compose stack"
	@echo "  make stop     Stop local Docker Compose stack"
	@echo "  make restart  Restart local Docker Compose stack"
	@echo "  make log      Tail local Compose logs"

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
