# JWT size examples

Este diretorio tem um gerador local para criar JWTs falsos com tamanho final
exato de 1KiB e 4KiB:

```bash
go run ./examples/jwt_size_examples.go
```

Para gravar os tokens em arquivos:

```bash
go run ./examples/jwt_size_examples.go -write
```

Isto cria:

- `examples/jwt-1024.txt`
- `examples/jwt-4096.txt`

O script imprime:

- tamanho final do token;
- tamanho do header `Authorization: Bearer ...`;
- tamanho do payload JSON antes de base64url;
- quantidade de roles geradas;
- arquivo gravado, quando `-write` for usado.

Para imprimir o token completo no terminal:

```bash
go run ./examples/jwt_size_examples.go -show-token
```

Notas:

- os tokens usam uma secret de exemplo e nao devem ser usados como credenciais;
- `Authorization: Bearer ` adiciona 7 bytes alem do JWT;
- o campo `pad` existe apenas para ajustar o tamanho exato do exemplo;
- as roles sao ilustrativas para visualizar quanto espaco elas ocupam;
- os arquivos `.txt` sao gravados sem quebra de linha final para que `wc -c`
  bata exatamente com o tamanho do JWT.
