# Projetos, ações e sessões do Kitty

O Konen guarda a rotina pessoal de cada workspace no estado versionável da
máquina, sem espalhar arquivos de Kitty ou do editor em todos os repositórios.

## Fluxo básico

Dentro do projeto, execute:

```console
konen project add
```

O assistente pergunta um nome curto, a pasta, um shell opcional, ações nomeadas
e uma ou mais abas. Uma aba vazia abre o shell de login na pasta do projeto. Um
`command` direto é passado ao shell interativo com `-lic`, de modo que ele veja
o ambiente do usuário; `hold = true` mantém a aba aberta depois que o comando
termina. O primeiro cadastro pode ficar só com a aba `Terminal`: editores como
o Neovim nunca são presumidos.

Use o projeto pela pasta atual ou pelo nome:

```console
konen projects
konen dev
konen dev my-app
konen my-app
konen dev my-app --dry-run
```

`konen NOME` é a forma curta de `konen dev NOME` para um projeto já
cadastrado; pastas arbitrárias nunca são registradas implicitamente. `konen
projects` é o comando principal de listagem. `konen project list` permanece
como alias compatível.

Dentro do Kitty, o Konen usa o controle remoto para adicionar abas à janela
atual e focar a primeira que criou. A aba invocadora permanece aberta por
padrão. Isso exige `allow_remote_control yes` no `kitty.conf`. Fora do Kitty,
ele produz uma sessão nativa temporária e abre uma nova janela.

## Ações são tarefas do mise

Uma ação é apenas um nome pessoal e estável para uma tarefa já declarada pelo
projeto. Por exemplo, o repositório pode conter:

```toml
# mise.toml do projeto
[tasks.test]
run = "go test ./..."

[tasks.console]
run = "docker compose exec web sh"
```

O manifesto do Konen aponta para essas tarefas; ele não copia seus comandos nem
cria funções escondidas:

O fluxo guiado grava `projects/NOME.toml` no estado do Konen:

```toml
version = 2
path = "~/Documents/Projects/my-app"
keep_invoking_tab = false

[actions.checks]
task = "test"

[actions.web-console]
task = "console"

[[tabs]]
title = "Neovim"
command = "nvim ."

[[tabs]]
title = "Checks"
action = "checks"
hold = true

[[tabs]]
title = "Console"
action = "web-console"

[[tabs]]
title = "Terminal"
```

Tanto `konen run my-app checks` quanto a aba `Checks` chamam a mesma
operação nativa, `mise run --raw test`, dentro do projeto. Uma aba usa `action`
ou `command`; os dois juntos são recusados. Dentro da pasta cadastrada, omita o
projeto:

```console
konen run checks --dry-run
konen run checks
```

`konen project run my-app checks` é a forma equivalente sob o grupo
`project`. O `--dry-run` mostra pasta, ação, tarefa e aprovação, sem chamar o
mise. O `mise.toml` do projeto continua sendo a fonte da tarefa reproduzível;
o manifesto central contém somente seus nomes pessoais e a disposição das
abas.

`keep_invoking_tab` vale apenas dentro do Kitty e usa `true` por padrão.
`false` fecha o terminal que invocou `konen dev` depois que todas as novas abas
abriram e a primeira recebeu foco; se ele for o único terminal da aba, o Kitty
fecha essa aba também.

Edite o cadastro pelo assistente com `konen project edit NOME`. `show`, `list`
e `--dry-run` são comandos de inspeção. A listagem e os planos também informam
se a aprovação local ainda vale ou precisa de revisão.

O inteiro `version` pertence somente ao manifesto do Konen, não ao código do
projeto. Quando o formato evolui, `konen migrate --dry-run` mostra o diff e
`konen migrate` cria um backup local antes da substituição. Uma versão futura
é recusada, e todo manifesto migrado precisa de nova aprovação.

## Confiança

O manifesto contém comandos diretos e nomes de tarefas executáveis. Sua
aprovação é local, vinculada ao SHA-256 exato do arquivo e não é versionada com
o estado. O assistente aprova o arquivo que acabou de gravar; uma edição manual
ou um pull invalida a aprovação:

```console
konen project show my-app
konen project trust my-app
```

O Konen não abre abas nem executa ações antes dessa aprovação. Uma ação ainda é
implementada pelo `mise.toml` do projeto, então o mise também pode pedir
`mise trust` quando esse arquivo for novo ou tiver mudado. As duas aprovações
protegem superfícies diferentes; o Konen não contorna a confiança do mise.
