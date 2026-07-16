# Pontos de entrada (`cmd`)

Cada subdiretório gera um executável:

| Comando | Responsabilidade |
| --- | --- |
| `go run ./cmd/api` | Monta Model, View e Controllers e inicia o servidor. |
| `go run ./cmd/dburl` | Exibe a URL PostgreSQL formada pela configuração. |
| `go run ./cmd/appurl` | Exibe o endereço local da interface web. |

## Onde o MVC começa

`cmd/api/main.go` é o ponto de entrada. Ele:

1. carrega e valida a configuração;
2. cria o logger e o contexto encerrado por sinais;
3. abre o pool PostgreSQL;
4. cria o Model;
5. cria os Controllers;
6. carrega a View web;
7. monta as rotas e inicia o servidor.

O arquivo mostra a montagem completa de propósito. Ele não contém regras de
clientes ou autenticação; essas regras pertencem ao Model.
