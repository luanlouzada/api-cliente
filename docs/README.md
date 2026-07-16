# Contrato HTTP (`docs`)

`openapi.yaml` descreve o contrato público da API: rotas, autenticação Bearer,
parâmetros, DTOs, códigos de resposta e exemplos.

O arquivo deve permanecer alinhado aos Controllers e DTOs. Ele não expõe tipos
internos do Model, como hash de senha ou metadados usados somente pelo banco.

Ao alterar uma rota:

1. atualize método, caminho, segurança e parâmetros;
2. atualize os esquemas de entrada e saída;
3. documente todos os status que o Controller pode devolver;
4. preserve `additionalProperties: false` quando campos extras forem rejeitados;
5. mantenha exemplos sem segredos reais.

O contrato é mantido manualmente para que a leitura não dependa de geração de
código ou de anotações específicas de uma biblioteca.
