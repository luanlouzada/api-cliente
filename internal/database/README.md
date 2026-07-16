# Banco de dados (`internal/database`)

Este pacote cria o conjunto de conexões PostgreSQL reutilizadas pelo Model. Esse
conjunto é chamado de *pool* e evita abrir uma conexão nova em cada requisição.

`NewPostgresPool` interpreta a URL, configura limites de conexões, reciclagem e
verificação periódica e executa `Ping` antes de devolver o pool. Assim, a API não
começa a aceitar requisições sem confirmar que o banco está acessível.

O pacote não contém regras de cliente nem consultas SQL. As consultas pertencem
ao Model; aqui existe somente a preparação da conexão compartilhada.
