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

A função `main` cria o logger e concentra a única saída explícita do processo.
`startApplication` carrega a configuração e prepara o encerramento por sinais;
`run` conecta o MVC, inicia o servidor e devolve qualquer erro. Essa divisão
permite que as funções internas retornem normalmente e executem seus `defer`
antes de o processo terminar.

A montagem permanece concentrada nesse arquivo para tornar visível como as
partes do MVC são conectadas. Regras de clientes e autenticação permanecem no
Model.
