# MVC dentro de `internal`

Esta pasta contém o código que pertence somente à aplicação. A estrutura usa
MVC diretamente, com DTO e Mapper como auxiliares da fronteira HTTP:

```text
internal/
├── model/       dados, regras, autenticação e PostgreSQL
├── controller/  rotas e Controllers HTTP
├── view/        JSON e interface web
├── dto/         formatos públicos da API
├── mapper/      conversões entre DTO e Model
├── config/      leitura e validação da configuração
└── database/    criação do pool PostgreSQL
```

## Fluxo permitido

```text
Controller → DTO → Mapper → Model
Controller → Mapper → View
Model → PostgreSQL
```

O Model não conhece HTTP. A View não decide autorização. O Mapper apenas copia
campos. O Controller não contém regra de negócio: ele interpreta a requisição,
chama o Model e escolhe a resposta.

`config` e `database` preparam a configuração e o pool de conexões usados pelo
MVC. Eles são inicializados em `cmd/api/main.go` antes de o servidor começar.

Para estudar uma operação completa, comece em `controller/router.go`, siga para
o método do Controller, atravesse o DTO e o Mapper, leia a operação correspondente
no Model e acompanhe o retorno até a View.
