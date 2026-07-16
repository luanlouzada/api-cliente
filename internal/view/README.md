# View (`internal/view`)

Esta é a camada **View** do MVC. Ela possui duas formas de apresentação:

- `json.go` serializa DTOs como respostas JSON;
- `frontend/` contém HTML, CSS e JavaScript da interface visual.

`assets.go` usa `go:embed`, recurso da linguagem que inclui arquivos no binário,
para colocar o frontend dentro do executável. A API pode servir `/` e
`/frontend/*` sem depender de arquivos externos durante a execução.

## Responsabilidades

A View pode:

- apresentar DTOs e mensagens públicas;
- coletar dados dos formulários;
- chamar os endpoints definidos no OpenAPI;
- manter o estado visual da sessão;
- sincronizar a sessão entre abas com Web Locks.

A View não pode decidir autorização, acessar o PostgreSQL, guardar hash de senha
ou substituir validações do Model.

## Sessão no navegador

A interface de demonstração guarda o par de tokens e seus metadados em uma única
chave do `localStorage`. A API Web Locks do navegador cria um bloqueio com nome
compartilhado e impede que duas abas rotacionem simultaneamente o mesmo refresh
token. Quando esse recurso não existe, a renovação automática é adiada para
evitar uma falsa detecção de reutilização.

O uso de `localStorage` é adequado apenas para esta demonstração local. Em uma
aplicação exposta, avalie refresh token em cookie `HttpOnly`, `Secure` e
`SameSite`, além de proteção contra falsificação de requisições entre sites
(CSRF).
