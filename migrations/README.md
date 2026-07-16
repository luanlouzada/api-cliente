# Migration PostgreSQL

Uma migration é um arquivo SQL que cria ou altera a estrutura do banco de forma
repetível. Este template começa com apenas uma versão, formada por dois arquivos:

- `000001_init.up.sql` cria diretamente o esquema completo;
- `000001_init.down.sql` desfaz essa criação na ordem inversa das dependências.

O projeto exige PostgreSQL 18 ou posterior porque usa a função nativa
`uuidv7()` para gerar os identificadores dos clientes.

## O que o esquema representa

O arquivo `up` cria primeiro `customers`, depois `refresh_token_families` e por
último `refresh_tokens`. Essa ordem é necessária porque uma família pertence a
um cliente, e cada token pertence a uma família.

As restrições do PostgreSQL também protegem regras importantes do Model:

- e-mail de cliente único e papel limitado a `customer` ou `admin`;
- datas de expiração posteriores à criação;
- hash do refresh token com os 32 bytes produzidos por SHA-256;
- exclusão das sessões quando o cliente é excluído;
- no máximo um refresh token ativo em cada família de sessão.

## Aplicar e reverter

```bash
make install-migrate
make migrate-up
```

Em um banco descartável de estudo, `make migrate-down` executa o arquivo `down`
e remove as três tabelas. Essa operação apaga os dados e não deve ser tratada
como estratégia de recuperação de produção.

Quando um projeto já publicado precisar mudar o esquema, não altere a migration
que já foi executada em outros bancos. Crie uma nova versão e revise seu efeito
com backup antes da implantação.
