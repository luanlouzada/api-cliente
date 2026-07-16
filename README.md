# Cliente API

API REST didática em Go para cadastro, autenticação e gerenciamento de clientes.
O projeto usa Chi para as rotas HTTP, PostgreSQL para persistência e pgx para a
conexão com o banco. As senhas são protegidas com bcrypt. A autenticação combina
tokens de acesso assinados no formato JSON Web Token (JWT) com credenciais de
renovação aleatórias e descartáveis.

## Arquitetura MVC

O projeto usa MVC de forma direta. DTOs e Mappers apenas protegem a fronteira
HTTP e não acrescentam novas camadas arquiteturais:

```text
Requisição HTTP → Controller → DTO de entrada → Mapper → Model
Model → Mapper → DTO de resposta → View JSON → Resposta HTTP
```

- [`internal/model`](internal/model/README.md): entidades, validações, regras,
  autenticação, transações e acesso ao PostgreSQL.
- [`internal/controller`](internal/controller/README.md): rotas, Controllers,
  autenticação HTTP, limites e escolha dos status.
- [`internal/view`](internal/view/README.md): respostas JSON e frontend.
- [`internal/dto`](internal/dto/README.md): formatos públicos de entrada e saída.
- [`internal/mapper`](internal/mapper/README.md): conversões entre DTO e Model.
- [`internal/config`](internal/config/README.md) e
  [`internal/database`](internal/database/README.md): configuração e conexão
  PostgreSQL, que dão suporte ao MVC sem formar novas camadas.
- [`cmd/api`](cmd/README.md): ponto de entrada que monta o MVC e inicia o servidor.

### Controller e Handler em Go

Os tipos `AuthenticationController`, `CustomerController` e
`FrontendController` são os Controllers deste projeto. Seus métodos recebem a
requisição, leem o DTO, chamam o Model e escolhem a View devolvida ao cliente.

A biblioteca padrão de Go chama de `http.Handler` qualquer componente capaz de
atender HTTP. Por isso, os métodos dos Controllers possuem a assinatura usada
por `http.HandlerFunc`, mas **Handler não é uma camada adicional**: é apenas o
nome da interface HTTP da linguagem. A arquitetura continua sendo:

```text
Rota → Controller → Model → View
```

### Roteiro de estudo: por onde começar

Para entender o projeto, acompanhe primeiro uma requisição de cadastro:

1. Siga [Executar](#executar), abra a interface e cadastre um cliente.
2. Leia [`internal/README.md`](internal/README.md) para ver o papel de cada parte.
3. Abra [`cmd/api/main.go`](cmd/api/main.go) e observe onde Model, View e
   Controllers são criados.
4. Encontre `POST /auth/register` em
   [`internal/controller/router.go`](internal/controller/router.go).
5. Continue em `AuthenticationController.Register`, no arquivo
   [`internal/controller/authentication.go`](internal/controller/authentication.go).
6. Compare [`internal/dto`](internal/dto/README.md) com
   [`internal/mapper`](internal/mapper/README.md) para entender a conversão da entrada.
7. Siga para `AuthenticationModel.Register`, em
   [`internal/model/authentication_model.go`](internal/model/authentication_model.go).
8. Ainda em [`internal/model`](internal/model/README.md), acompanhe validação,
   bcrypt, criação dos tokens e transação PostgreSQL.
9. Volte pelo Mapper até [`internal/view/json.go`](internal/view/json.go), onde o
   DTO é serializado como resposta.
10. Repita o caminho com login, renovação da sessão e uma rota protegida de clientes.

O percurso completo do cadastro é:

```text
cmd/api/main.go → controller.NewRouter → AuthenticationController.Register
→ DTO → Mapper → AuthenticationModel.Register
→ regras e PostgreSQL no Model → Mapper → View JSON → resposta HTTP
```

### Glossário

- **Model:** representa os dados e executa validações, regras e persistência.
- **View:** apresenta o resultado; nesta API, é o JSON e a interface web.
- **Controller:** recebe a intenção HTTP, chama o Model e escolhe a View.
- **DTO:** define exatamente os campos aceitos ou devolvidos pela API.
- **Mapper:** converte DTOs e tipos do Model sem executar regras.
- **JWT:** formato de token com campos assinados. A assinatura prova que o
  conteúdo não foi alterado, mas não esconde esse conteúdo.
- **HS256:** algoritmo que usa a mesma chave secreta para assinar e validar o JWT.
- **bcrypt:** algoritmo de hash de senha que inclui um valor aleatório, chamado
  *salt*, e é propositalmente custoso para dificultar tentativas em massa.
- **Token de acesso:** JWT curto enviado no cabeçalho `Authorization`.
- **Refresh token:** valor aleatório e de uso único usado para renovar a sessão.
- **Bearer:** modo de enviar o token de acesso, como em
  `Authorization: Bearer <token>`.
- **Transação:** grupo de operações do banco confirmado por inteiro ou desfeito
  por inteiro quando algo falha.
- **Bloqueio:** proteção temporária de uma linha do banco para ordenar alterações
  que acontecem ao mesmo tempo.

## Pré-requisitos

- Go `1.26.4`, conforme o `go.mod`;
- PostgreSQL `18+` — a migration inicial usa `uuidv7()` nativo;
- Docker com Docker Compose para o banco local;
- GNU Make e um terminal compatível com POSIX (no Windows, use WSL);
- OpenSSL apenas para o comando sugerido de geração do segredo.

## Executar

Crie a configuração local:

```bash
cp .env.example .env
openssl rand -base64 48
```

Copie a saída do segundo comando para `JWT_SECRET` no `.env`. O exemplo não
fornece uma chave padrão válida, pois uma chave pública permitiria forjar JWTs.

Suba o PostgreSQL, aplique a migration e inicie a aplicação:

```bash
make install-migrate # somente na primeira vez
make up
```

A API e a interface web ficam em `http://127.0.0.1:8080`. Para executar as etapas
separadamente:

```bash
make db-up
make wait-db
make migrate-up
go run ./cmd/api
```

`JWT_SECRET` é obrigatório e deve possuir pelo menos 32 bytes. O token de acesso
deve durar ao menos 10 segundos para respeitar a precisão do JWT e a renovação
antecipada da interface web. Durações inválidas e portas fora do intervalo fazem a
aplicação falhar na inicialização, em vez de assumir um valor silenciosamente.
Por segurança, a API escuta somente `127.0.0.1` por padrão. Defina
`HTTP_HOST=0.0.0.0` conscientemente apenas quando precisar expô-la pela rede e
já tiver configurado o primeiro administrador, TLS e os controles do ambiente.

O Go e o Docker Compose leem o `.env`; o Makefile não tenta interpretar esse
arquivo de variáveis. As migrations obtêm a URL pelo mesmo pacote
`internal/config` usado pela API. Caracteres especiais do usuário, da senha e do
nome do banco são codificados para não mudar o significado da URL. Uma
`DATABASE_URL` explícita sobrescreve as variáveis `POSTGRES_*`. `make up` é o
atalho para o ambiente local; com banco externo, rode somente `make migrate-up`
e `make backend`.

## Endpoints

| Método | Rota | Autenticação | Descrição |
|---|---|---:|---|
| `POST` | `/auth/register` | Pública | Cadastra como `customer` e emite os tokens |
| `POST` | `/auth/login` | Pública | Autentica e emite os tokens |
| `POST` | `/auth/refresh` | Refresh token | Rotaciona o refresh token |
| `POST` | `/auth/logout` | Refresh token | Revoga toda a sessão de refresh |
| `POST` | `/cliente` | Bearer `admin` | Cria um cliente |
| `GET` | `/cliente` | Bearer `admin` | Lista clientes |
| `GET` | `/cliente/{id}` | Dono ou `admin` | Busca um cliente |
| `PUT` | `/cliente/{id}` | Dono ou `admin` | Atualiza um cliente |
| `DELETE` | `/cliente/{id}` | Dono ou `admin` | Exclui um cliente |

Erros usam um envelope estável:

```json
{
  "error": {
    "code": "validation_error",
    "message": "email deve ser válido"
  }
}
```

Corpo JSON inválido retorna `400`, conteúdo acima de 64 KiB retorna `413`,
`Content-Type` diferente de `application/json` retorna `415` e uma operação que
ultrapasse 10 segundos retorna `504`. Campos JSON desconhecidos são rejeitados.
Cadastro e login compartilham, por endereço IP, uma capacidade imediata de 20
requisições, reposta gradualmente à velocidade de 120 por minuto. Renovação e
logout usam outra capacidade de 20, reposta a 240 por minuto. O excesso retorna
`429` e informa em `Retry-After` quantos segundos aguardar.

### Regra de autorização deste exemplo

O cadastro público sempre cria o papel `customer`; o corpo não aceita `role`.
A regra de autorização fica no Model, e não apenas no Controller: `admin` pode
operar todo o catálogo, enquanto `customer` só pode ler, atualizar ou excluir o
próprio ID. Esse ID é comparado ao campo `sub` do JWT, abreviação de *subject*
(sujeito), que identifica para quem o token foi emitido. O papel é obrigatório;
papéis ausentes ou desconhecidos invalidam o token.

Em uma instalação nova ainda não existe administrador. Antes de expor a API na
rede, mantenha `HTTP_HOST=127.0.0.1`, cadastre localmente a conta administrativa,
copie o `customer.id` retornado e pare a API. Promova exatamente esse UUID, com
email e papel atual como condições adicionais (o comando usa as credenciais
padrão do exemplo):

```bash
docker compose exec -T postgres psql -U app -d app \
  -v ON_ERROR_STOP=1 \
  -c "UPDATE customers SET role = 'admin' \
      WHERE id = 'COLE-O-UUID-RETORNADO' \
        AND email = 'admin@example.com' \
        AND role = 'customer' \
      RETURNING id, email, role;"
```

Confirme que o comando retornou exatamente uma linha; zero linhas indica que os
dados não conferem e nada deve ser promovido. Depois reinicie a API e faça login
ou refresh para obter o novo papel. Ajuste usuário e banco no comando se alterou
o `.env`.

## Sessões e refresh token

O token de acesso dura 15 minutos por padrão. O refresh token:

- tem 256 bits aleatórios e prefixo `rt_`;
- é persistido somente como hash SHA-256;
- é de uso único e rotacionado a cada renovação;
- possui limite de inatividade e limite absoluto da sessão;
- revoga a família inteira quando um token anterior é reutilizado;
- possui garantia no banco de no máximo um token ativo por família.

As operações de renovação, logout e reuso bloqueiam a mesma linha de família no
PostgreSQL e seguem uma ordem única de bloqueios com a linha do cliente. Isso
coloca operações simultâneas em uma ordem definida, evita que duas transações
fiquem esperando uma pela outra e impede que um token novo escape da revogação.
A validade é conferida pelo relógio do banco depois da espera pelos bloqueios,
inclusive na criação da sessão. Cadastro, login e renovação assinam o token de
acesso dentro da respectiva transação: se a assinatura falhar, cliente e sessão
novos não são confirmados, e uma rotação não consome o refresh token anterior.
O logout revoga refresh tokens, mas um token de
acesso já emitido continua válido até o campo `exp`, que registra sua expiração.

O papel também está no token de acesso autocontido. Uma promoção, um rebaixamento
ou a exclusão da conta não altera um token já emitido; ele conserva o papel até
expirar (15 minutos por padrão). Um sistema que exija revogação imediata deve
adicionar `token_version` consultada em armazenamento ou uma lista de bloqueio
compartilhada.

Famílias expiradas ou revogadas são mantidas para auditoria. Em uma operação de
produção, agende uma política de retenção. A chave estrangeira com
`ON DELETE CASCADE` faz o PostgreSQL excluir os tokens filhos junto com a família;
por exemplo, é possível remover famílias encerradas há mais de sete dias sem
deixar tokens órfãos:

```sql
DELETE FROM refresh_token_families
WHERE (revoked_at IS NOT NULL AND revoked_at < now() - interval '7 days')
   OR (revoked_at IS NULL AND expires_at < now() - interval '7 days');
```

O frontend embutido é uma ferramenta de demonstração e guarda os tokens no
`localStorage`, armazenamento persistente do navegador, para facilitar a
exploração local. Em uma aplicação web exposta à internet, prefira guardar o
refresh token em cookie `HttpOnly`, `Secure` e `SameSite`. Também adote proteção
contra falsificação de requisições entre sites, ataque conhecido pela sigla CSRF.

O limitador de requisições da demonstração é local a cada processo e usa o
endereço da conexão direta, sem confiar cegamente no cabeçalho
`X-Forwarded-For`. Em produção com múltiplas instâncias ou proxy reverso, use
armazenamento compartilhado e configure quais proxies podem informar o IP real.

O contrato completo da API está em [`docs/openapi.yaml`](docs/openapi.yaml).
