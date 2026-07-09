ifneq (,$(wildcard .env))
include .env
export
endif

PORT ?= 8080
POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5432
POSTGRES_USER ?= app
POSTGRES_PASSWORD ?= app
POSTGRES_DB ?= app
POSTGRES_SSLMODE ?= disable
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)
GOCACHE ?= /tmp/go-build-cache

.PHONY: help up start dev db-up wait-db migrate-up backend frontend frontend-url down db-down logs install-migrate test

help:
	@printf "Comandos principais:\n"
	@printf "  make up              Sobe banco, aplica migrations e roda backend + frontend\n"
	@printf "  make db-up           Sobe apenas o Postgres\n"
	@printf "  make migrate-up      Aplica migrations no banco configurado\n"
	@printf "  make backend         Roda a API Go, servindo o frontend em /\n"
	@printf "  make frontend        Mostra a URL do frontend\n"
	@printf "  make frontend-url    Mostra a URL do frontend\n"
	@printf "  make down            Para e remove containers do compose\n"
	@printf "  make test            Roda os testes Go\n"

up: db-up wait-db migrate-up backend

start: up

dev: up

db-up:
	@docker compose up -d postgres

wait-db:
	@printf "Aguardando Postgres"
	@i=0; \
	until docker compose exec -T postgres pg_isready -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" >/dev/null 2>&1; do \
		i=$$((i + 1)); \
		if [ $$i -gt 30 ]; then \
			printf "\nPostgres nao ficou pronto em 30 segundos.\n"; \
			exit 1; \
		fi; \
		printf "."; \
		sleep 1; \
	done; \
	printf " ok\n"

migrate-up:
	@command -v migrate >/dev/null || { \
		printf "migrate nao encontrado. Rode: make install-migrate\n"; \
		exit 1; \
	}
	@migrate -path migrations -database "$(DATABASE_URL)" up

backend:
	@printf "Backend:  http://localhost:$(PORT)\n"
	@printf "Frontend: http://localhost:$(PORT)/\n"
	@GOCACHE="$(GOCACHE)" go run main.go

frontend: frontend-url

frontend-url:
	@printf "Frontend: http://localhost:$(PORT)/\n"

down:
	@docker compose down

db-down:
	@docker compose stop postgres

logs:
	@docker compose logs -f postgres

install-migrate:
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

test:
	@GOCACHE="$(GOCACHE)" go test ./...
