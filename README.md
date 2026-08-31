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

Konen não é outro gerenciador de pacotes. Ele cuida da experiência inicial,
segurança, diagnóstico e sessões de projetos; mise continua sendo a fonte da
verdade sobre o conteúdo da máquina.

## Instalação

A versão de teste atual precisa ser informada porque prereleases não são
selecionadas automaticamente:

```console
curl -fsSLO https://raw.githubusercontent.com/roqem/konen/main/install.sh
less install.sh
KONEN_VERSION=v0.1.0-alpha.6 sh install.sh
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
2. `status` compara o que foi declarado com o que existe na máquina;
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
| **Manifesto de projeto** | Arquivo TOML que descreve a pasta e as abas usadas por `konen dev`. |
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
└── projects/      # sessões de projetos usadas por konen dev
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

Use `[tools]` para programas de desenvolvimento que mise sabe gerenciar:

```toml
[tools]
node = "lts"
neovim = "latest"
ruby = "3.4"
```

`latest` acompanha a versão mais recente; uma versão como `3.4` limita a
atualização à família escolhida. `konen status` mostra a versão pedida, a versão
resolvida e se ela já está instalada.

### Pacotes do sistema

Bibliotecas de compilação e programas fornecidos pela distribuição pertencem a
`[bootstrap.packages]`:

```toml
[bootstrap.packages]
"apt:build-essential" = { os = "linux" }
"apt:libssl-dev" = { os = "linux" }
"apt:zsh" = { os = "linux" }
```

O prefixo informa o gerenciador (`apt`, `dnf`, `pacman`, `brew` etc.). Esse é o
momento em que mise pode chamar `sudo`, sempre depois de a operação aparecer no
plano.

### Repositórios auxiliares

Use `[bootstrap.repos]` para checkouts que fazem parte do ambiente, mas não são
seus projetos de trabalho:

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

Crie `~/home/scripts/bin/work-note`:

```sh
#!/bin/sh
set -eu
printf '%s\n' "${1:-Lembrete sem texto}"
```

Depois:

```console
chmod +x ~/home/scripts/bin/work-note
konen trust
konen status
```

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
backup; cloná-los em outra máquina os restaura.

### Instaladores pessoais

Prefira `[tools]`, `[bootstrap.packages]`, `[bootstrap.repos]` e `[dotfiles]`.
Crie um instalador somente quando o software exigir passos que essas seções não
conseguem expressar, como adicionar um repositório oficial do fornecedor.

Um arquivo `~/home/mise-tasks/install/example` pode começar assim:

```sh
#!/bin/sh
#MISE description="Instala o aplicativo Example"
set -eu

if command -v example >/dev/null 2>&1; then
  exit 0
fi

# Coloque aqui os menores passos necessários, usando fontes oficiais.
```

Marque-o como executável:

```console
chmod +x ~/home/mise-tasks/install/example
```

Para executá-lo automaticamente durante `konen apply`, selecione a tarefa no
`mise.toml`:

```toml
[tasks.bootstrap]
run = [
  { task = "install:example" },
]
```

Use referências em `run` para manter instaladores sequenciais. Dependências
comuns podem rodar em paralelo e dois instaladores que usam apt disputariam o
mesmo lock.

Finalize revisando o próprio script:

```console
git -C ~/home diff
konen trust
konen plan
konen apply
```

`plan` mostra que a tarefa foi selecionada, mas não a executa. Tarefas pessoais
são chamadas em toda aplicação; por isso cada uma deve detectar o que já está
pronto e terminar rapidamente. Evite `curl ... | sh`: prefira repositórios
assinados ou artefatos oficiais, valide o que foi baixado e deixe cada uso de
`sudo` visível no arquivo.

O guia técnico, incluindo a fronteira exata de confiança, está em
[docs/automation.md](docs/automation.md).

## Fazendo backup com Git

O backup é o próprio repositório do estado. Antes do primeiro commit, confirme
que não há senhas, tokens, chaves privadas, `.env` ou dados específicos que não
deveriam sair da máquina:

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

O assistente pergunta o nome, a pasta, o shell e as abas. Uma aba pode abrir
`nvim .`, subir Docker, iniciar Codex/Claude ou apenas deixar um terminal. Os
manifestos ficam em `~/home/projects`, portanto também entram no backup.

`konen dev` encontra o projeto pela pasta atual. De qualquer outro lugar, use
`konen dev my-app` ou o atalho `konen my-app`. Comandos de projeto têm uma
aprovação local separada; uma edição ou pull exige
`konen project trust NOME`. Veja [docs/projects.md](docs/projects.md).

## Comandos do dia a dia

| Comando | O que faz |
|---|---|
| `konen` | Abre o menu interativo. |
| `konen status` | Mostra o que existe, falta ou está diferente. |
| `konen plan` | Simula a aplicação completa sem alterar a máquina. |
| `konen diff` | Mostra diferenças nos dotfiles gerenciados. |
| `konen apply` | Aplica o estado pedindo confirmações. |
| `konen apply --yes` | Aplica sem perguntas; use somente depois de revisar o plano. |
| `konen trust` | Aprova localmente o estado executável que você revisou. |
| `konen dotfile add CAMINHO` | Passa a gerenciar uma configuração existente. |
| `konen project add [DIR]` | Cadastra um projeto e suas abas. |
| `konen projects` | Lista projetos e a situação da aprovação. |
| `konen dev [NOME]` | Abre as abas do projeto no Kitty. |
| `konen doctor` | Diagnostica instalação, estado, confiança, mise e Git. |
| `konen completion SHELL` | Gera autocomplete para Zsh, Bash ou Fish. |

Repetir `apply` é parte normal do fluxo. Recursos já convergidos são ignorados;
instaladores pessoais idempotentes são chamados, verificam o estado e saem sem
refazer trabalho.

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

O autocomplete inclui comandos, opções, caminhos e nomes de projetos.

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
Symlinks nessa superfície executável são recusados. Estados criados pelo Konen
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
