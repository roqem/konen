# Roadmap

Este roadmap registra direção e critérios de conclusão, não datas. Enquanto o
Konen estiver em alpha, a ordem pode mudar conforme os testes em máquinas reais
revelarem problemas mais importantes.

## Princípios

- manter mise e formatos conhecidos como fonte da verdade;
- oferecer um caminho guiado sem esconder os arquivos gerados;
- mostrar comandos e mudanças antes de executá-los;
- preservar a possibilidade de editar tudo manualmente;
- não guardar credenciais nem automatizar commits e pushes;
- adicionar abstrações somente para problemas observados no uso real.

## Fundação — concluída

A primeira fundação pública já cobre:

- instalação sem root, checksums, releases e CI;
- criação ou clone de um estado em qualquer pasta;
- autenticação assistida para estados privados no GitHub;
- confiança local sensível ao conteúdo executável;
- status, plano, diff e aplicação delegados ao mise;
- pacotes, ferramentas, repositórios, dotfiles e tarefas pessoais;
- comandos pessoais versionados;
- projetos aprovados e abertos em abas do Kitty;
- menu, autocomplete e diagnóstico;
- bootstrap completo e reaplicação idempotente numa VM Ubuntu 26.04 limpa.

## Marco 1 — edição guiada do estado

Objetivo: permitir que alguém monte seu primeiro estado sem precisar conhecer
TOML, mantendo o `mise.toml` resultante simples e totalmente editável.

- adicionar pelo menu ferramentas, pacotes do sistema e repositórios;
- explicar versão, plataforma e gerenciador antes de gravar;
- reutilizar o fluxo existente para capturar dotfiles;
- criar ou importar um comando pessoal em `scripts/bin`;
- criar o esqueleto de um instalador e selecioná-lo no `bootstrap`, sem gerar
  comandos de instalação não revisados;
- mostrar o diff do arquivo que será alterado;
- atualizar a confiança após uma mutação feita pelo assistente;
- orientar a criação do primeiro commit e avisar quando o estado ainda não tem
  remoto, sem fazer commit ou push automaticamente.

Critério de conclusão: uma pessoa iniciante consegue adicionar uma ferramenta,
um pacote, um dotfile e um comando pessoal pelo menu, revisar o plano, aplicar e
restaurar o mesmo estado em outra máquina. O resultado continua sendo estado
nativo do mise, sem manifesto paralelo do Konen.

## Marco 2 — rotina diária e evolução

Objetivo: tornar manutenção e atualização tão claras quanto o primeiro uso.

- resumir ao final do `apply` o que mudou e quais ações manuais restaram;
- distinguir melhor recursos convergidos de tarefas idempotentes sempre
  selecionadas;
- oferecer filtros de status para itens ausentes, diferentes ou por categoria;
- projetar atualização explícita do Konen e do mise, com versão e plano
  visíveis;
- definir migrações compatíveis para qualquer mudança futura do estado;
- modelar ações nomeadas de projeto, como preparar, testar, abrir console ou
  gerar cobertura, sem transformá-las em funções escondidas do Konen;
- permitir que abas reutilizem essas ações em vez de duplicar comandos longos.

Critério de conclusão: o usuário entende o resultado de cada aplicação, atualiza
o produto conscientemente e mantém rotinas de projeto no estado central sem
depender de aliases obscuros.


## Marco 3 — beta pública

Objetivo: estabilizar o contrato antes de ampliar distribuição e integrações.

- automatizar um smoke test público em ambiente Linux descartável;
- manter a qualificação manual com um estado privado representativo;
- testar instalação, atualização, interrupção de rede e reaplicação;
- revisar mensagens e documentação com linguagem direta e natural;
- documentar plataformas suportadas e limites conhecidos;
- definir política de compatibilidade e migração de versões;
- publicar a primeira beta somente quando um estado criado numa alpha puder ser
  atualizado sem reconstrução manual.

Depois da beta, entram em avaliação pacote `.deb`, repositório APT assinado,
outros terminais e perfis de máquina. Eles não devem atrasar a estabilização do
fluxo central.
