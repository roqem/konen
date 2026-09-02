# Konen

> Do zero à sua máquina.

Konen é uma interface pequena e guiada para reconstruir seu ambiente de
desenvolvimento a partir de arquivos comuns, legíveis e versionáveis. Ele é um
projeto [Roqem](https://github.com/roqem); o nome vem do hebraico כּוֹנֵן
(*konen*), estabelecer ou construir.

Você descreve o que deseja ter — ferramentas, pacotes, configurações, projetos
e automações pessoais — e o Konen ajuda a revisar, aplicar e restaurar esse
estado em outra máquina. O trabalho de instalação e sincronização é feito pelo
[mise](https://mise.jdx.dev/), incluído pelo instalador quando necessário.

Não é preciso conhecer mise para começar. Este README apresenta o fluxo do
zero; os documentos em [`docs/`](docs/) aprofundam as decisões técnicas.

## Situação do projeto

Konen está em **alpha**: o fluxo completo já foi validado em uma VM Ubuntu
26.04 limpa, mas comandos e formato ainda podem mudar antes da primeira versão
estável. A versão mínima do mise aceita pelo contrato atual é 2026.8.15.

Os próximos marcos estão em [docs/roadmap.md](docs/roadmap.md).
Plataformas qualificadas, limites conhecidos e a política de migração estão em
[docs/support.md](docs/support.md).

Konen não é outro gerenciador de pacotes. Ele cuida da experiência inicial,
segurança, diagnóstico e sessões de projetos; mise continua sendo a fonte da
verdade sobre o conteúdo da máquina.

## Instalação

A versão de teste atual precisa ser informada porque prereleases não são
selecionadas automaticamente:

```console
curl -fsSLO https://raw.githubusercontent.com/roqem/konen/main/install.sh
less install.sh
KONEN_VERSION=v0.1.0-alpha.23 sh install.sh
export PATH="$HOME/.local/bin:$PATH"
konen version
```

O instalador não usa `sudo`. Ele verifica os checksums e coloca `konen` e uma
versão compatível do `mise` em `~/.local/bin`. Os programas que seu estado
pedir mais tarde podem precisar de `sudo`. Recursos declarativos aparecem por
inteiro em `konen plan`; tarefas personalizadas aparecem pelo nome e pelo
caminho do script que deve ser revisado.

Depois da primeira versão estável, a entrada curta será:

```console
curl -fsSL https://raw.githubusercontent.com/roqem/konen/main/install.sh | sh && ~/.local/bin/konen
```

Detalhes sobre versões fixas, espelhos e atestações estão em
[docs/distribution.md](docs/distribution.md).

### Deixando Konen e mise ativos no terminal

O `export PATH=...` usado acima vale apenas para o terminal atual. Para Zsh,
adicione estas linhas ao `~/.zshrc`:

```zsh
export PATH="$HOME/.local/bin:$PATH"
eval "$(mise activate zsh)"
eval "$(konen completion zsh)"
```

Para Bash, adicione-as ao `~/.bashrc`, trocando o nome do shell:

```bash
export PATH="$HOME/.local/bin:$PATH"
eval "$(mise activate bash)"
eval "$(konen completion bash)"
```

A ativação do mise coloca as ferramentas e os comandos pessoais declarados no
`PATH`; a última linha habilita o autocomplete do Konen. Abra um terminal novo
depois de salvar. Você também pode guardar esse próprio arquivo de shell como
dotfile, tornando a configuração parte do backup.

## Começando do zero

Escolha uma pasta para guardar seu estado. `~/home` é apenas um exemplo; o
nome e o caminho são seus:

```console
konen init --git ~/home
konen status
konen plan
konen apply
```

Esses comandos fazem o seguinte:

1. `init` cria a estrutura inicial e, com `--git`, inicia um repositório Git;
2. `status` compara o que foi declarado com o que existe na máquina e mostra a
   situação do backup Git;
3. `plan` mostra o que uma aplicação faria, sem alterar nada;
4. `apply` pede confirmação e aproxima a máquina do estado declarado.

O estado inicial é propositalmente pequeno. Acrescente suas ferramentas e
configurações aos poucos usando os exemplos abaixo. O Konen nunca cria commits
nem envia mudanças ao GitHub por você.

Executar apenas `konen` abre um menu com as mesmas ações. `q`, Escape e Ctrl+C
fecham o menu.

## Restaurando um backup existente

Se o estado já estiver num repositório GitHub, uma máquina nova começa assim:

```console
konen init --from github:SEU_USUARIO/home ~/home
less ~/home/mise.toml
konen trust
konen status
konen plan
konen apply
```

Um repositório privado abre um login por código de dispositivo. O navegador
pode estar em outro aparelho; não é preciso criar uma chave SSH antes. Konen
usa a credencial somente no contexto desse clone e não altera a configuração
Git global.

O `trust` é obrigatório porque o estado remoto pode executar tarefas e colocar
comandos no seu `PATH`. Ele registra uma aprovação **local** do conteúdo exato
que você revisou. Um `git pull` que altere `mise.toml`, comandos ou tarefas exige
nova revisão e novo `konen trust`.

## Vocabulário essencial

| Termo | Significado no Konen |
|---|---|
| **Estado** | A pasta que descreve sua máquina e pode virar um repositório Git. |
| **mise** | O motor que instala ferramentas e converge pacotes, repositórios e arquivos. |
| **Convergir** | Fazer o estado real se aproximar do estado declarado sem repetir trabalho já concluído. |
| **Dotfile** | Arquivo de configuração do usuário, como `~/.zshrc` ou `~/.config/nvim`. |
| **Ferramenta** | Programa de usuário com versão gerenciada, como Node, Ruby ou Neovim. |
| **Pacote do sistema** | Pacote instalado por apt, dnf, pacman, Homebrew ou equivalente. |
| **Tarefa** | Script versionado que mise consegue executar. |
| **Instalador pessoal** | Tarefa idempotente usada quando uma instalação não cabe nas declarações normais. |
| **Manifesto de projeto** | Arquivo TOML que descreve pasta, ações pessoais e abas usadas por `konen run` e `konen dev`. |
| **Trust / confiança** | Aprovação local do código que aquele estado ou projeto pode executar. |

`Idempotente` significa que executar novamente é seguro: a automação verifica
primeiro se o resultado já existe e só faz o que ainda falta.

## O que existe dentro do estado

Uma pasta criada pelo Konen começa com esta estrutura:

```text
home/
├── .git/          # existe quando --git foi usado
├── .gitignore     # evita credenciais e arquivos secretos comuns
├── mise.toml      # ferramentas, pacotes, repos, dotfiles e tarefas escolhidas
├── home/          # cópias/fontes dos dotfiles gerenciados
├── mise-tasks/
│   └── install/   # instaladores pessoais executáveis
├── scripts/
│   └── bin/       # comandos pessoais adicionados ao PATH
└── projects/      # ações e sessões usadas por konen run/dev
```

Konen guarda apenas o caminho dessa pasta em
`~/.config/konen/config.toml`. O `mise.toml` do estado também vira a configuração
global do mise, permitindo usar as ferramentas declaradas dentro de qualquer
projeto. Um `mise.toml` local de projeto ainda pode escolher versões diferentes.

### `mise.toml` sem mistério

TOML organiza dados em seções entre colchetes. Cada seção deve aparecer uma só
vez no arquivo; se `[tools]` já existir, acrescente as novas linhas dentro dela
em vez de criar outro `[tools]`.

O ciclo seguro depois de uma edição manual é:

```console
git -C ~/home diff
konen trust
konen plan
konen apply
```

## Personalizando sua máquina

### Ferramentas com versão

O assistente pergunta o nome e a versão, mostra o diff do `mise.toml` e pede
confirmação antes de gravar:

```console
konen tool add
```

Os mesmos campos podem ser informados diretamente. A prévia não grava nem
instala nada:

```console
konen tool add --dry-run node lts
konen tool add node lts
```

Depois da gravação, execute `konen plan` e `konen apply` para instalar. Em
scripts não interativos, `--yes` confirma somente a edição do estado:

```console
konen tool add --yes node lts
```

O resultado continua sendo a seção nativa `[tools]` do mise:

```toml
[tools]
node = "lts"
neovim = "latest"
ruby = "3.4"
```

`latest` acompanha a versão mais recente; uma versão como `3.4` limita a
atualização à família escolhida. `konen status` mostra a versão pedida, a versão
resolvida e se ela já está instalada.

### Filtrando o status

O status completo é útil para auditoria. Na rotina diária, ele pode ser reduzido
por categoria, situação ou pelas duas ao mesmo tempo:

```console
konen status --only tools,dotfiles
konen status --state pending
konen status --state missing,different
konen status --only tools --state missing
```

As categorias são `packages`, `repos`, `dotfiles`, `tools`, `task` e `user`.
As situações têm significado estável:

| Situação | Inclui |
|---|---|
| `ready` | Recursos já convergidos e comandos pessoais disponíveis. |
| `pending` | Tudo que exige aplicação ou correção conhecida. |
| `missing` | Recursos ausentes, inclusive dotfiles cuja fonte não existe. |
| `different` | Recursos existentes que diferem do estado declarado. |
| `unknown` | Estados novos que o Konen ainda não classifica. |

Valores separados por vírgula são combinados. Categoria e situação funcionam
em conjunto. A orientação de `Backup Git` aparece apenas no status completo,
para não misturar recursos fora do filtro.

### Escolhendo o que aplicar

O plano completo continua disponível sem perguntas com `konen plan`. Para
revisar apenas algumas etapas, abra o seletor:

```console
konen plan --select
```

Os checkboxes trabalham por etapa — pacotes, repositórios, dotfiles,
ferramentas e tarefas pessoais — para preservar as dependências internas de
cada grupo. O mesmo seletor pode iniciar uma aplicação parcial:

O mise chama esse conjunto ordenado de etapas de *bootstrap*. Você verá esse
nome no `mise.toml` e na saída original do mise; no Konen, ele corresponde ao
plano e à aplicação do estado.

```console
konen apply --select
```

Em scripts, servidores ou outros terminais não interativos, informe as etapas
explicitamente:

```console
konen plan --only tools,dotfiles
konen apply --only tools,dotfiles
```

Durante essas operações, o Konen limita o mise ao estado atualmente
configurado. Uma configuração global anterior ou arquivos de versão encontrados
em pastas ancestrais não são misturados ao plano.

Depois de um `apply` bem-sucedido, o Konen consulta o estado antes e
depois da aplicação e resume:

- quais recursos convergiram naquela execução;
- quais já estavam prontos;
- o que ainda está pendente, inclusive em etapas não selecionadas;
- se a tarefa `bootstrap` apenas terminou sem erro, sem fingir que uma tarefa
  idempotente possui estado convergente;
- ações observáveis, como iniciar uma nova sessão após mudar o shell de login.

Instaladores pessoais continuam responsáveis por mostrar instruções próprias,
como autenticação ou reinício. O resumo manda revisar essas mensagens. Se uma
tarefa alterar o `mise.toml`, outra tarefa ou um comando pessoal durante o
`apply`, o Konen não faz a consulta posterior: exige nova revisão com
`konen trust`.

### Pacotes do sistema

Bibliotecas de compilação e programas fornecidos pela distribuição pertencem a
[`[bootstrap.packages]`](https://mise.jdx.dev/bootstrap/packages/). O assistente
detecta o gerenciador mais provável, explica em qual plataforma ele é usado e
mostra o diff antes de gravar:

```console
konen package add
konen package add --dry-run --manager apt jq latest
konen package add --manager apt jq latest
```

O cadastro não instala nada. Depois de gravar, revise apenas essa etapa com
`konen plan --only packages`; a instalação acontece somente num `apply` que
inclua pacotes. Em automações, `--yes` confirma apenas a edição do estado.

O resultado continua sendo configuração nativa do mise:

```toml
[bootstrap.packages]
"apt:build-essential" = "latest"
"apt:libssl-dev" = "latest"
"apt:zsh" = "latest"
```

O prefixo informa o gerenciador (`apt`, `dnf`, `pacman`, `brew`, `flatpak`
etc.); o valor informa a versão pedida. Mise pode chamar `sudo` durante a
aplicação de alguns gerenciadores, sempre depois de a operação aparecer no
plano.

### Repositórios auxiliares

Use [`[bootstrap.repos]`](https://mise.jdx.dev/bootstrap/repos.html) para
checkouts que fazem parte do ambiente, mas não são seus projetos de trabalho. O
caminho deve ser absoluto ou começar com `~/`:

```console
konen repo add
konen repo add --dry-run ~/.oh-my-zsh https://github.com/ohmyzsh/ohmyzsh.git
konen repo add ~/.oh-my-zsh https://github.com/ohmyzsh/ohmyzsh.git
```

Assim como os outros assistentes, o comando mostra o diff e não clona nada.
Use `konen plan --only repos` para revisar o clone antes do `apply`. Informar
uma branch, tag ou commit torna a referência desejada explícita; sem `REF`, um
checkout que já existe não é atualizado automaticamente.

```toml
[bootstrap.repos]
"~/.oh-my-zsh" = { url = "https://github.com/ohmyzsh/ohmyzsh.git" }
```

Projetos nos quais você trabalha normalmente não precisam estar aqui; eles
podem ser clonados à parte e cadastrados com `konen project add`.

### Dotfiles

Para começar a guardar uma configuração existente:

```console
konen dotfile add ~/.zshrc
konen dotfile add --mode copy ~/.config/kitty/kitty.conf
konen diff
```

Os modos são:

- `symlink`: padrão; o arquivo usado pelo programa aponta para a fonte dentro
  do estado, então uma edição já aparece no Git;
- `copy`: `apply` copia a fonte para o destino; use `konen diff` para perceber
  mudanças feitas diretamente no destino;
- `template`: modo avançado para gerar conteúdo diferente por máquina.

O comando copia a configuração para `home/` e atualiza `[dotfiles]` no
`mise.toml`. Arquivos conhecidos por conter credenciais, chaves SSH privadas e
o `~/.gitconfig` inteiro são recusados. Preferências Git portáveis podem ficar
em `~/.config/git/config`.

### Comandos pessoais

Um comando pessoal é um arquivo executável e versionado. Ele é melhor
que um alias quando contém lógica, precisa receber argumentos ou será usado por
uma aba de projeto.

O assistente pode criar um esqueleto inofensivo:

```console
konen command add --dry-run work-note
konen command add work-note
```

O arquivo criado apenas avisa que ainda precisa ser implementado e termina com
erro; o Konen nunca o executa durante o cadastro. Depois da confirmação, edite
o caminho exibido pelo assistente.

Também é possível importar um comando que já existe. O nome é opcional e, se
omitido, vem do arquivo de origem:

```console
konen command add --dry-run --from ~/bin/work-note
konen command add --from ~/bin/work-note
konen command add --from ~/bin/work-note work-note
```

A importação mostra o conteúdo completo, copia os bytes para `scripts/bin` e
torna a cópia executável. Para que a prévia permaneça segura e o resultado seja
portátil, a origem deve ser um arquivo de texto UTF-8 com shebang; links
simbólicos e arquivos especiais são recusados.

O resultado ainda é apenas um arquivo comum. Por exemplo,
`~/home/scripts/bin/work-note` pode conter:

```sh
#!/bin/sh
set -eu
printf '%s\n' "${1:-Lembrete sem texto}"
```

Depois de implementar o esqueleto, aprove novamente o conteúdo que mudou:

```console
konen trust
konen status
```

Se preferir criar o arquivo inteiramente à mão, marque-o como executável com
`chmod +x` antes do `konen trust`.

Abra um terminal no qual mise esteja ativo e execute:

```console
work-note "Revisar o pull request"
```

O estado inicial já inclui esta configuração, responsável por adicionar a
pasta ao `PATH` quando mise está ativo:

```toml
[env]
_.path = "{{ config_source | canonicalize | dirname }}/scripts/bin"
```

Scripts em `scripts/bin` são arquivos normais do Git. Fazer commit deles é o
backup; cloná-los em outra máquina os restaura. Um alias curto e puramente
interativo ainda pode ficar no arquivo do shell; quando houver lógica ou
reutilização por projetos, prefira um comando pessoal.

### Instaladores pessoais

Prefira `[tools]`, `[bootstrap.packages]`, `[bootstrap.repos]` e `[dotfiles]`.
Crie um instalador somente quando o software exigir passos que essas seções não
conseguem expressar, como adicionar um repositório oficial do fornecedor.

O assistente cria o arquivo e já o seleciona, em ordem, no bootstrap:

```console
konen installer add --dry-run example
konen installer add example
```

A prévia mostra o arquivo executável completo e o diff do `mise.toml`. Nada é
executado durante o cadastro. O esqueleto criado não contém comandos de
instalação e termina com erro de propósito: implemente e revise o arquivo antes
do próximo `apply`, em vez de deixar uma instalação vazia parecer bem-sucedida.

Um `~/home/mise-tasks/install/example` implementado pode ficar assim:

```sh
#!/bin/sh
#MISE description="Instala o aplicativo Example"
set -eu

if command -v example >/dev/null 2>&1; then
  exit 0
fi

# Coloque aqui os menores passos necessários, usando fontes oficiais.
```

Depois de editar o esqueleto, aprove o conteúdo executável que mudou:

```console
git -C ~/home diff
konen trust
konen plan --only task
```

O assistente já terá criado esta seleção no `mise.toml`:

```toml
[tasks.bootstrap]
run = [
  { task = "install:example" },
]
```

Use referências em `run` para manter instaladores sequenciais. Dependências
comuns podem rodar em paralelo e dois instaladores que usam apt disputariam o
mesmo lock.

Também é possível importar um instalador já existente. O nome é opcional e, se
omitido, vem do arquivo de origem:

```console
konen installer add --dry-run --from ~/bin/install-example
konen installer add --from ~/bin/install-example
konen installer add --from ~/bin/install-example example
```

A importação copia exatamente o texto revisado, exige um arquivo regular UTF-8
com shebang e grava a cópia como executável. Links simbólicos e arquivos
especiais são recusados.

`plan` mostra que a tarefa foi selecionada, mas não a executa. Quando o plano
estiver correto, `konen apply` a chamará depois dos recursos declarativos e das
ferramentas. Tarefas pessoais são chamadas em toda aplicação; por isso cada
uma deve detectar o que já está pronto e terminar rapidamente. Evite
`curl ... | sh`: prefira repositórios assinados ou artefatos oficiais, valide o
que foi baixado e deixe cada uso de `sudo` visível no arquivo.

O guia técnico, incluindo a fronteira exata de confiança, está em
[docs/automation.md](docs/automation.md).

## Fazendo backup com Git

O backup é o próprio repositório do estado. Antes do primeiro commit, confirme
que não há senhas, tokens, chaves privadas, `.env` ou dados específicos que não
deveriam sair da máquina:

```console
konen status
```

A seção `Backup Git` informa se o repositório foi iniciado, se o primeiro
commit ainda está pendente, se há mudanças locais e quais remotos existem.
Quando algo falta, ela mostra os próximos comandos, mas não executa nenhum
deles. Endereços de remotos não são exibidos, pois podem conter informações de
autenticação.

O fluxo manual indicado continua sendo:

```console
cd ~/home
git status
git diff
git add .
git diff --cached
git commit -m "configura meu ambiente"
```

Se o GitHub CLI (`gh`) já estiver instalado, ele oferece o caminho mais curto
para criar um repositório privado:

```console
gh auth login
gh repo create home --private --source=. --remote=origin --push
```

Também é possível criar um repositório vazio pelo site do GitHub, autenticar o
Git pelo método indicado pela plataforma e conectar o endereço exibido:

```console
git remote add origin https://github.com/SEU_USUARIO/home.git
git push -u origin main
```

No uso diário, você decide quando salvar:

```console
git -C ~/home status
git -C ~/home diff
git -C ~/home add .
git -C ~/home commit -m "atualiza meu ambiente"
git -C ~/home push
```

O repositório privado é o padrão mais prudente porque caminhos, nomes de
projetos e preferências pessoais podem revelar informações. Mesmo num
repositório privado, credenciais não devem ser versionadas. O `.gitignore`
inicial ajuda, mas não substitui a revisão de `git diff --cached`.

## Projetos e abas do terminal

Konen pode guardar como cada projeto é aberto sem espalhar arquivos de Kitty,
Neovim ou assistentes por todos os repositórios:

```console
cd ~/Documents/Projects/my-app
konen project add
konen projects
konen dev --dry-run
konen dev
```

O assistente pergunta o nome, a pasta, o shell, ações nomeadas e abas. Uma ação
é um nome pessoal para uma tarefa que o próprio projeto já declara no mise. Por
exemplo, `checks` pode apontar para a tarefa `test`; `konen run my-app checks` e
uma aba com `action = "checks"` executam a mesma tarefa, sem copiar seu comando
para o manifesto. Abas ainda podem abrir um comando direto, como `nvim .`, ou
apenas deixar um terminal. Os manifestos ficam em `~/home/projects`, portanto
também entram no backup.

`konen dev` encontra o projeto pela pasta atual. De qualquer outro lugar, use
`konen dev my-app` ou o atalho `konen my-app`. Comandos de projeto têm uma
aprovação local separada; uma edição ou pull exige
`konen project trust NOME`. A tarefa continua no `mise.toml` do projeto e
também respeita a confiança do mise. Veja [docs/projects.md](docs/projects.md).

## Comandos do dia a dia

| Comando | O que faz |
|---|---|
| `konen` | Abre o menu interativo. |
| `konen status` | Mostra o que existe, falta ou está diferente. |
| `konen status --only tools,dotfiles` | Mostra somente as categorias informadas. |
| `konen status --state missing,different` | Mostra recursos ausentes ou diferentes. |
| `konen plan` | Simula a aplicação completa sem alterar a máquina. |
| `konen plan --select` | Escolhe por checkboxes quais etapas entram no plano. |
| `konen diff` | Mostra diferenças nos dotfiles gerenciados. |
| `konen apply` | Aplica o estado pedindo confirmações. |
| `konen apply --select` | Escolhe por checkboxes quais etapas serão aplicadas. |
| `konen apply --only tools,dotfiles` | Aplica somente as etapas informadas. |
| `konen apply --yes` | Aplica sem perguntas; use somente depois de revisar o plano. |
| `konen migrate --dry-run` | Mostra mudanças necessárias nos formatos do Konen. |
| `konen migrate [--yes]` | Migra formatos antigos depois da revisão. |
| `konen update --dry-run` | Mostra versões e mecanismos sem atualizar. |
| `konen update [--yes]` | Atualiza componentes próprios após revisão. |
| `konen trust` | Aprova localmente o mise.toml, as tarefas e os comandos revisados. |
| `konen tool add [NOME] [VERSÃO]` | Adiciona uma ferramenta ao estado mostrando o diff. |
| `konen package add [--manager M] PACOTE [VERSÃO]` | Adiciona um pacote do sistema sem instalá-lo. |
| `konen repo add DESTINO URL [REF]` | Adiciona um checkout Git sem cloná-lo. |
| `konen command add [NOME]` | Cria um comando pessoal sem executá-lo. |
| `konen command add --from ARQUIVO [NOME]` | Importa um comando existente. |
| `konen installer add [NOME]` | Cria e seleciona um instalador pessoal sem executá-lo. |
| `konen installer add --from ARQUIVO [NOME]` | Importa e seleciona um instalador existente. |
| `konen dotfile add CAMINHO` | Passa a gerenciar uma configuração existente. |
| `konen project add [DIR]` | Cadastra um projeto, suas ações e abas. |
| `konen projects` | Lista projetos e a situação da aprovação. |
| `konen run [PROJETO] AÇÃO` | Executa uma ação nomeada pela tarefa do mise. |
| `konen dev [NOME]` | Abre as abas do projeto no Kitty. |
| `konen doctor` | Diagnostica instalação, estado, confiança, mise e Git. |
| `konen completion SHELL` | Gera autocomplete para Zsh, Bash ou Fish. |

Repetir `apply` é parte normal do fluxo. O resumo distingue recursos que já
estavam prontos de tarefas pessoais: recursos convergidos são ignorados;
instaladores idempotentes são chamados, verificam o estado e saem sem refazer
trabalho.

## Autocomplete

Para Zsh, adicione ao `~/.zshrc`:

```zsh
eval "$(konen completion zsh)"
```

Para Bash, adicione ao `~/.bashrc`:

```bash
eval "$(konen completion bash)"
```

Para Fish:

```fish
konen completion fish > ~/.config/fish/completions/konen.fish
```

O autocomplete inclui comandos, opções, caminhos, nomes de projetos e ações.

## Compatibilidade e migrações

O Konen versiona somente os formatos que ele próprio define: a configuração
local e os manifestos em `projects/`. `mise.toml`, tarefas, comandos pessoais e
dotfiles continuam nos formatos de suas ferramentas e não recebem uma versão
paralela do Konen.

Uma versão antiga é detectada antes do uso. Revise todas as mudanças sem gravar:

```console
konen migrate --dry-run
```

A tabela mostra a versão encontrada e a suportada, seguida pelo diff de cada
arquivo. Para aplicar depois da revisão:

```console
konen migrate
# ou, em automações:
konen migrate --yes
```

Antes de alterar qualquer arquivo, o Konen confirma que nada mudou durante a
revisão e guarda os originais em `~/.config/konen/migration-backups/` — ou no
diretório equivalente sob `XDG_CONFIG_HOME`. As gravações são atômicas; uma
falha restaura os arquivos já alterados. Manifestos migrados perdem a aprovação
local e precisam de um novo `konen project trust NOME`.

Se um arquivo tiver sido criado por uma versão mais nova do Konen, ele não é
alterado: atualize o programa primeiro. Arquivos locais de aprovação são caches
regeneráveis e não fazem parte do estado portável.

## Atualizando o Konen e o mise

Atualização é uma operação separada do estado da máquina. Primeiro consulte o
plano sem trocar nenhum executável:

> `konen update` existe desde a alpha.15. Versões anteriores precisam usar o
> `install.sh` mais uma vez; depois disso, as próximas atualizações podem ser
> feitas pelo próprio Konen.

```console
konen update --dry-run
konen update --dry-run --only konen
konen update --dry-run --only mise
```

A tabela mostra a versão atual, a release publicada escolhida e o mecanismo que
será usado. Builds prerelease, como as alphas, acompanham a prerelease mais
recente. Builds estáveis ignoram prereleases; `--pre` permite incluí-las
explicitamente.

Quando o plano estiver correto:

```console
konen update
# ou, em automações:
konen update --yes
```

O Konen baixa seu archive, verifica o checksum publicado e valida a versão do
executável preparado antes da substituição atômica. O mise coinstalado ao lado
dele é atualizado pelo próprio `mise self-update`, sem atualizar plugins.
Downloads e validações do Konen terminam antes da atualização do mise.

O auto-update só substitui um Konen instalado dentro da pasta pessoal e um mise
coinstalado no mesmo diretório. Se APT, Homebrew ou outro gerenciador for o
responsável, o plano mostra o caminho e pede que a atualização seja feita por
ele. O repositório de estado, seus dotfiles e projetos não são alterados.

## Segurança em poucas regras

- sempre leia `konen plan` antes do primeiro `apply` numa máquina;
- leia `mise.toml`, comandos e tarefas antes de executar `konen trust`;
- nunca versione tokens, senhas, cookies, chaves privadas ou arquivos `.env`;
- use fontes oficiais e HTTPS nos instaladores;
- mantenha comandos privilegiados explícitos;
- revise `git diff --cached` antes de cada primeiro push importante.

Konen calcula um digest do `mise.toml`, de todos os diretórios de tarefas mise
reconhecidos e de `scripts/bin`, incluindo as permissões executáveis. Qualquer
mudança bloqueia operações respaldadas por mise até uma nova aprovação.
Links simbólicos nesses caminhos são recusados. Estados criados pelo Konen
são aprovados localmente; estados clonados ou adotados de outro lugar não são.

Mais detalhes estão em [docs/architecture.md](docs/architecture.md).

## Desenvolvimento do Konen

O próprio repositório usa mise:

```console
mise install
mise run check
```

Sem mise, use Go 1.27.0 ou mais recente:

```console
go test ./...
go build ./cmd/konen
```

Critérios de release e o teste numa VM limpa estão em
[docs/testing.md](docs/testing.md).

## Licença

Apache-2.0.
