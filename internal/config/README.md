# Configuração (`internal/config`)

Este pacote lê `.env` e variáveis do processo, aplica valores padrão seguros e
valida tudo antes de abrir conexões ou iniciar HTTP.

`Load` devolve a configuração completa da API. `LoadDatabaseURL` e
`LoadFrontendURL` atendem os pequenos comandos em `cmd` usando as mesmas regras.

Uma `DATABASE_URL` explícita possui prioridade. Na ausência dela, a URL é formada
pelas variáveis `POSTGRES_*`, codificando caracteres especiais de usuário, senha
e nome do banco para que não alterem o significado da URL.

Configurações inválidas interrompem a inicialização: porta fora do intervalo,
host ambíguo, segredo JWT curto e durações de sessão incoerentes não recebem
correção silenciosa.
