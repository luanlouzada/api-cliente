# api-cliente

API REST em Go para gerenciamento de clientes.

## Endpoints

- `POST /cliente` — criar cliente
- `GET /cliente` — listar clientes
- `GET /cliente/{id}` — buscar por id
- `PUT /cliente/{id}` — atualizar cliente
- `DELETE /cliente/{id}` — remover cliente

## Executar

Configure o ambiente:

```bash
cp .env.example .env
set -a
source .env
set +a
```

Instale o `golang-migrate` pelo Go:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Garanta que os binarios instalados pelo Go estao no `PATH`.
Esse comando vale apenas para o terminal atual:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Se quiser deixar permanente no Bash:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
source ~/.bashrc
```

Confirme se instalou:

```bash
migrate -version
```

Suba o banco e aplique a migration:

```bash
docker compose up -d postgres
DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=${POSTGRES_SSLMODE}"
migrate -path migrations -database "$DATABASE_URL" up
```

Veja a versão aplicada:

```bash
migrate -path migrations -database "$DATABASE_URL" version
```

Desfaça só a última migration:

```bash
migrate -path migrations -database "$DATABASE_URL" down 1
```

Inicie a API:

```bash
go run main.go
```

A API sobe em `http://localhost:$PORT`.
