# Model (`internal/model`)

Esta é a camada **Model** do MVC. Ela reúne o estado e o comportamento da
aplicação: entidades, validações, autorização, autenticação, criptografia,
transações e leitura ou escrita no PostgreSQL.

O Model não importa Controller, DTO, Mapper ou View. Assim, uma regra de cliente
não depende de JSON, rota ou status HTTP.

## Organização

| Arquivo | Responsabilidade |
| --- | --- |
| `customer.go` | Entidade cliente e validações de perfil e senha. |
| `customer_model.go` | Criação, listagem, consulta, atualização e exclusão. |
| `customer_data.go` | Comandos PostgreSQL de clientes. |
| `authentication_model.go` | Cadastro, login, renovação e logout. |
| `authentication_data.go` | Transações e bloqueios das sessões. |
| `access_token.go` | Emissão e validação de JWT. |
| `refresh_token_manager.go` | Geração e hash de refresh tokens. |
| `refresh_token.go` | Estado persistido da sessão. |
| `password.go` | Hash e comparação de senhas com bcrypt. |
| `errors.go` | Erros de negócio reconhecidos pelo Controller. |

## Regras importantes

- cadastro público sempre cria `customer`;
- somente `admin` cria ou lista outros clientes;
- um `customer` acessa apenas o próprio cadastro;
- senha em texto puro nunca é persistida;
- refresh token é salvo somente como hash e usado uma vez;
- reutilização revoga toda a família da sessão;
- transações usam uma ordem única de bloqueios para ordenar alterações simultâneas;
- o relógio do PostgreSQL decide a validade depois da aquisição dos bloqueios.

As consultas ficam no mesmo pacote porque persistência faz parte do Model neste
MVC. O Controller enxerga apenas as operações públicas de `CustomerModel` e
`AuthenticationModel`.
