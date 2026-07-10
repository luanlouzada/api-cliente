# api-cliente

API REST em Go para gerenciamento de clientes.

## Endpoints

- `POST /auth/register` — cadastrar e emitir access/refresh tokens
- `POST /auth/login` — autenticar e emitir access/refresh tokens
- `POST /auth/refresh` — renovar e rotacionar os tokens
- `POST /auth/logout` — revogar a sessao do refresh token
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

## Renovar o token

O cadastro e o login retornam `access_token`, `expires_at`, `refresh_token`,
`refresh_expires_at` e `session_expires_at`. O refresh token e opaco, salvo no
banco somente como hash SHA-256 e rotacionado em todo uso.

Por padrao, o access token dura 15 minutos, o refresh token expira apos 7 dias
sem renovacao e a familia da sessao termina definitivamente depois de 30 dias.
Uma rotacao nunca estende `session_expires_at`.

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"rt_..."}'
```

Substitua sempre o refresh token anterior pelo novo valor retornado. A tentativa
de reutilizar um token ja rotacionado revoga toda a familia da sessao.
