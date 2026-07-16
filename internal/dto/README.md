# DTOs (`internal/dto`)

DTO significa **Data Transfer Object**. Os tipos desta pasta definem exatamente
os campos JSON aceitos e devolvidos pela API.

DTO não é entidade do Model. Por exemplo, `CustomerResponse` não possui hash de
senha, e `UpdateCustomerRequest` não permite alterar papel ou credencial.

## Arquivos

| Arquivo | Conteúdo |
| --- | --- |
| `customer.go` | Entrada de criação, entrada de atualização e saída de cliente. |
| `authentication.go` | Login, refresh token e resposta de sessão. |
| `error.go` | Envelope uniforme de erro. |

O Controller decodifica os DTOs de entrada. O Mapper converte entre DTO e Model.
A View serializa os DTOs de saída.
