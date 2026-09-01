# Suporte e compatibilidade

Este documento separa o que o Konen entrega hoje do que apenas pode funcionar.
Enquanto o projeto estiver em alpha, essa distinção é mais útil do que uma
promessa genérica de “suporte a Linux”.

## Plataformas qualificadas

| Ambiente | Situação | O que é verificado |
| --- | --- | --- |
| Linux `amd64` | suportado na alpha | CI, arquivos de release e instalação pública num home descartável |
| Ubuntu 26.04 `amd64` | ambiente de referência | jornada manual completa numa VM limpa, incluindo estado privado e reaplicação |
| Linux `arm64` | experimental | o arquivo de release é compilado, recebe checksum e atestação, mas ainda não passa pelo smoke numa máquina `arm64` |
| macOS e Windows | não suportados | não há arquivos de release, instalador nem qualificação |

O executável e o instalador evitam dependências específicas de uma distribuição,
mas o estado da máquina pode pedi-las. Uma declaração `apt:jq`, por exemplo,
continua sendo própria de Debian e Ubuntu. O mise também entende outros
gerenciadores; o assistente conseguir gravar uma declaração não significa que
essa combinação já foi qualificada pelo Konen.

O instalador público precisa de um shell POSIX e dos comandos `curl`, `tar`,
`awk`, `sha256sum`, `install`, `mktemp`, `mkdir` e `uname`. Ele instala apenas
Konen e mise, sem `sudo`. Git é necessário para criar ou clonar um estado
versionado. Programas declarados pelo usuário podem ter requisitos e pedir
privilégios próprios, sempre visíveis no plano ou no instalador pessoal.

Zsh, Bash e Fish recebem autocomplete. A maior parte da CLI independe do shell.
Sessões com abas exigem Kitty; ações de projeto, inspeção, migração e aplicação
do estado não exigem Kitty.

## Limites conhecidos

- `konen apply` não é uma transação de sistema. Pacotes, repositórios, arquivos
  e tarefas podem produzir efeitos antes de uma etapa posterior falhar. A
  reaplicação deve ser segura, mas o Konen não promete desfazer mudanças feitas
  por APT, Git, mise ou scripts pessoais.
- Tarefas e instaladores pessoais são código arbitrário. O Konen mostra sua
  localização, vincula a confiança ao conteúdo e bloqueia alterações não
  aprovadas; ele não consegue provar que um script é seguro ou idempotente.
- Uma queda de rede interrompe instalação ou atualização depois de tentativas
  limitadas. Binários funcionais e o estado são preservados, mas não existe modo
  offline nem retomada parcial de downloads.
- O fluxo assistido de autenticação privada conhece GitHub. Outros provedores e
  remotos SSH dependem de credenciais já configuradas pelo usuário.
- O Konen não guarda segredos, não cria commits ou remotos e não faz push. Ele
  orienta o backup Git, mas a revisão e o envio continuam explícitos.
- Aplicativos gráficos e mudanças que exigem novo login, como shell de login ou
  participação no grupo Docker, precisam de verificação manual depois do
  `apply`.
- Não há desinstalador, pacote `.deb`, repositório APT, perfis de máquina ou
  integração com terminais diferentes do Kitty nesta fase.

## Quais arquivos têm versão do Konen

O Konen possui somente dois formatos próprios e versionados:

| Arquivo | Versão atual | Versões antigas migráveis |
| --- | ---: | --- |
| configuração local `config.toml` | 1 | 0, que não tinha o campo `version` |
| manifesto `projects/*.toml` | 2 | 0 e 1 |

O `mise.toml`, tarefas do mise, comandos pessoais e dotfiles permanecem em seus
formatos nativos. O Konen não adiciona outra versão a esses arquivos e não os
reescreve durante `konen migrate`. Registros de confiança são caches locais e
podem ser reconstruídos; não fazem parte do estado portável.

## Política de migração

Uma versão nova segue estas regras:

1. formato atual é lido sem alteração;
2. formato antigo conhecido é recusado até uma migração explícita;
3. `konen migrate --dry-run` mostra todos os diffs e não grava nada;
4. `konen migrate` cria backups privados, confirma que os arquivos não mudaram
   durante a revisão e usa substituição atômica;
5. uma falha de gravação detectada restaura arquivos já migrados;
6. manifestos de projeto migrados perdem a aprovação local;
7. formato criado por um Konen mais novo nunca é rebaixado: o programa orienta
   atualizar primeiro.

A primeira beta só será publicada se configurações v1 e projetos v1/v2 criados
pelas alphas públicas puderem chegar ao formato da beta sem reconstrução manual.
Da primeira beta até a 1.0, todo formato publicado terá leitura direta ou uma
migração explícita para a versão seguinte. Arquivos experimentais nunca
publicados e modificações locais fora do formato documentado não entram nessa
garantia.

Durante a alpha, comandos e opções ainda podem mudar. Na beta, mudanças
incompatíveis de CLI precisam vir com mensagem acionável e caminho de transição;
aliases baratos podem permanecer temporariamente. A partir da 1.0, versões
seguem versionamento semântico: incompatibilidades exigem uma nova versão
principal.

## Como verificar antes de atualizar

```console
konen doctor
konen update --dry-run
konen migrate --dry-run
```

Atualize o executável antes de migrar arquivos. Depois da migração, revise os
diffs e renove apenas as aprovações indicadas. O procedimento completo de
qualificação está em [testing.md](testing.md), e as decisões de distribuição
estão em [distribution.md](distribution.md).
