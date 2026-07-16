# Mappers (`internal/mapper`)

Mapper é o conversor entre os DTOs da fronteira HTTP e os tipos do Model.

```text
DTO de entrada → Mapper → entrada do Model
resultado do Model → Mapper → DTO de resposta
```

O Mapper copia e seleciona campos. Ele não consulta banco, não decide
autorização, não normaliza dados e não aplica regra de negócio.

## Arquivos

| Arquivo | Responsabilidade |
| --- | --- |
| `customer.go` | Converte entradas e saídas de clientes. |
| `authentication.go` | Converte login, cadastro e resposta de sessão. |

A separação impede que campos internos, como `PasswordHash`, sejam incluídos em
JSON por acidente e evita que alterações no contrato HTTP modifiquem diretamente
as entidades do Model.
