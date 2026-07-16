# Controller (`internal/controller`)

Esta é a camada **Controller** do MVC. Ela recebe HTTP, interpreta a entrada,
chama o Model e escolhe a View devolvida ao cliente.

## Controller e `http.Handler`

Em Go, `http.Handler` e `http.HandlerFunc` são os tipos da biblioteca padrão
usados para atender requisições. Neste projeto, isso não cria uma camada
“Handler”: os métodos de `AuthenticationController`, `CustomerController` e
`FrontendController` são os próprios Controllers e possuem a assinatura aceita
pelo roteador.

Um *middleware* é apenas uma função que envolve um `http.Handler`: ela pode agir
antes e depois do próximo atendimento. Aqui os middlewares cuidam de log,
autenticação, prazo, limites e cabeçalhos, sem assumir regras do Model.

## Arquivos principais

| Arquivo | Responsabilidade |
| --- | --- |
| `router.go` | Associa métodos e caminhos aos Controllers. |
| `authentication.go` | Cadastro, login, renovação e logout via HTTP. |
| `customer.go` | Operações HTTP de clientes. |
| `frontend.go` | Entrega a View web. |
| `json.go` | Decodifica JSON de entrada com limites estritos. |
| `response.go` | Converte erros do Model em status públicos. |
| `middleware.go` | Autenticação Bearer, logs, recuperação e cabeçalhos. |
| `timeout.go` | Impõe prazo sem publicar uma resposta tardia. |
| `rate_limit.go` | Limita tentativas por endereço IP direto. |

## Fluxo de um método

Um Controller deve seguir esta ordem:

1. ler parâmetros, identidade e DTO;
2. usar o Mapper para formar a entrada do Model;
3. chamar o Model;
4. traduzir erros conhecidos para HTTP;
5. usar Mapper e View para escrever a resposta.

Validação de formato pertence ao Controller. Validação de negócio e autorização
pertencem ao Model.
