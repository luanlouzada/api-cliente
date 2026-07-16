GOCACHE ?= /tmp/go-build-cache
TOOLS_DIR ?= $(CURDIR)/.bin
MIGRATE ?= $(TOOLS_DIR)/migrate
MIGRATE_VERSION ?= v4.19.1

.PHONY: help up start dev db-up wait-db migrate-up migrate-down backend frontend frontend-url down db-down logs install-migrate build fmt fmt-check vet test test-web test-race test-integration cover check

help:
	@printf "Comandos principais:\n"
	@printf "  make up              Sobe o banco, aplica migrações e executa a API\n"
	@printf "  make db-up           Sobe apenas o Postgres\n"
	@printf "  make migrate-up      Aplica migrações no banco configurado\n"
	@printf "  make backend         Executa a API Go e serve a interface web em /\n"
	@printf "  make frontend        Mostra a URL da interface web\n"
	@printf "  make frontend-url    Mostra a URL da interface web\n"
	@printf "  make down            Para e remove containers do compose\n"
	@printf "  make check           Executa formatação, vet, testes Go/web e compilação\n"
	@printf "  make test            Executa os testes unitários Go\n"
	@printf "  make test-web        Valida e testa o JavaScript da interface\n"
	@printf "  make test-integration Roda os testes PostgreSQL (requer TEST_DATABASE_URL)\n"

up: db-up wait-db migrate-up backend

start: up

dev: up

db-up:
	@docker compose up -d postgres

wait-db:
	@printf "Aguardando Postgres"
	@i=0; \
	until docker compose exec -T postgres sh -c 'pg_isready -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' >/dev/null 2>&1; do \
		i=$$((i + 1)); \
		if [ $$i -gt 30 ]; then \
			printf "\nPostgres não ficou pronto em 30 segundos.\n"; \
			exit 1; \
		fi; \
		printf "."; \
		sleep 1; \
	done; \
	printf " ok\n"

migrate-up:
	@test -x "$(MIGRATE)" || { \
		printf "migrate não encontrado. Execute: make install-migrate\n"; \
		exit 1; \
	}
	@database_url="$${DATABASE_URL:-}"; \
	if [ -z "$$database_url" ]; then \
		database_url="$$(GOCACHE="$(GOCACHE)" go run ./cmd/dburl)" || exit 1; \
	fi; \
	"$(MIGRATE)" -path migrations -database "$$database_url" up

migrate-down:
	@test -x "$(MIGRATE)" || { \
		printf "migrate não encontrado. Execute: make install-migrate\n"; \
		exit 1; \
	}
	@database_url="$${DATABASE_URL:-}"; \
	if [ -z "$$database_url" ]; then \
		database_url="$$(GOCACHE="$(GOCACHE)" go run ./cmd/dburl)" || exit 1; \
	fi; \
	"$(MIGRATE)" -path migrations -database "$$database_url" down 1

backend:
	@GOCACHE="$(GOCACHE)" go run ./cmd/api

frontend: frontend-url

frontend-url:
	@GOCACHE="$(GOCACHE)" go run ./cmd/appurl

down:
	@docker compose down

db-down:
	@docker compose stop postgres

logs:
	@docker compose logs -f postgres

install-migrate:
	@mkdir -p "$(TOOLS_DIR)"
	@GOBIN="$(TOOLS_DIR)" go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

build:
	@GOCACHE="$(GOCACHE)" go build ./...

fmt:
	@find . -type d \( -name .git -o -name vendor \) -prune -o \
		-type f -name '*.go' -print0 | xargs -0 -r gofmt -w

fmt-check:
	@files="$$(find . -type d \( -name .git -o -name vendor \) -prune -o \
		-type f -name '*.go' -print0 | xargs -0 -r gofmt -l)"; \
	if [ -n "$$files" ]; then \
		printf "Arquivos fora do gofmt:\n%s\n" "$$files"; \
		exit 1; \
	fi

vet:
	@GOCACHE="$(GOCACHE)" go vet ./...

test:
	@GOCACHE="$(GOCACHE)" go test -count=1 ./...

test-web:
	@node --check internal/view/frontend/app.js
	@if [ -f internal/view/app_test.js ]; then \
		node --check internal/view/app_test.js && \
		node --test internal/view/app_test.js; \
	fi

test-race:
	@GOCACHE="$(GOCACHE)" go test -race -count=1 ./...

test-integration:
	@test -n "$${TEST_DATABASE_URL:-}" || { \
		printf "Defina TEST_DATABASE_URL para um banco PostgreSQL descartável.\n"; \
		exit 1; \
	}
	@GOCACHE="$(GOCACHE)" go test -race -tags=integration -count=1 ./internal/model

cover:
	@GOCACHE="$(GOCACHE)" go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

check: fmt-check vet test-race test-web build
