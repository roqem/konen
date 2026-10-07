# Projetos, ações e sessões do Kitty

O Konen guarda a rotina pessoal de cada workspace no estado versionável da
máquina, sem espalhar arquivos de Kitty ou do editor em todos os repositórios.

## Fluxo básico

Dentro do projeto, execute:

```console
konen project add
```

O assistente pergunta um nome curto, a pasta e um shell opcional; em seguida,
abre o menu de abas e ações. Uma aba vazia abre o shell de login na pasta do
projeto. Um `command` direto é passado ao shell interativo com `-lic`, de modo que ele veja
o ambiente do usuário. Quando o comando termina, a aba volta a um shell
utilizável na mesma pasta e posição, onde você pode executar o comando outra
vez. Isso usa o recurso nativo
[`launch --hold` do Kitty](https://sw.kovidgoyal.net/kitty/launch/#cmdoption-launch-hold).
Use `hold = false` quando quiser fechá-la automaticamente. O
primeiro cadastro pode ficar só com a aba `Terminal`: editores como o Neovim
nunca são presumidos.

Use o projeto pela pasta atual ou pelo nome:

```console
konen projects
konen project
konen dev
konen dev my-app
konen my-app
konen dev my-app --dry-run
konen dev my-app --tab "Terminal"
```

`konen NOME` é a forma curta de `konen dev NOME` para um projeto já
cadastrado; pastas arbitrárias nunca são registradas implicitamente. `konen
projects` é o comando principal de listagem. `konen project list` permanece
como alias compatível.

`konen project` abre o gerenciador: selecione um projeto para abrir a sessão
completa, abrir somente uma aba, executar uma ação ou editar sua configuração.
O menu principal de `konen` também oferece essa entrada. `--tab TÍTULO` abre
somente a aba escolhida e mantém a aba invocadora; pode ser combinado com
`--dry-run` e com o atalho `konen NOME`.

Dentro do Kitty, o Konen usa o controle remoto para adicionar abas à janela
atual e focar a primeira que criou. A aba invocadora permanece aberta por
padrão. Isso exige `allow_remote_control yes` no `kitty.conf`. Fora do Kitty,
ele produz uma sessão nativa temporária e abre uma nova janela.

## Editar abas e ações

```console
konen project edit
konen project edit my-app
```

Sem o nome, o Konen usa o projeto da pasta atual ou oferece uma seleção. O menu
mostra as abas na ordem em que serão abertas e as ações com suas tarefas. Você
pode escolher diretamente o item que quer alterar:

- uma aba permite editar título, comando e comportamento ao sair, mover para
  cima ou para baixo e remover;
- uma nova aba pode abrir um shell, comando direto, ação cadastrada ou tarefa
  escolhida na lista do mise;
- uma ação permite trocar o nome ou a tarefa e abrir sua implementação no
  Neovim. Renomear a ação atualiza as abas que a utilizam; remover uma ação em
  uso exige ajustar essas abas primeiro.

O comportamento ao sair é uma escolha explícita entre **Voltar ao shell**
(padrão) e **Fechar a aba**. `hold` ausente em cadastros existentes também usa
o novo padrão; um `hold = false` explícito continua sendo respeitado. As sessões
já abertas não mudam: a escolha vale ao abrir novas abas.

**Salvar abas e ações** grava o cadastro e sua aprovação local. Escape ou
**Descartar alterações** preserva o cadastro anterior. Escape dentro de um
formulário volta ao menu, sem aplicar aquela edição. Alterações na ordem não
movem abas de uma sessão que já esteja aberta.

## Ver e editar tarefas do mise

```console
konen project tasks my-app
konen project task edit my-app
konen project task edit my-app test
```

A listagem mostra nome, descrição, comando, escopo e arquivo de origem das
tarefas que o mise resolve na pasta cadastrada, inclusive tarefas globais.
Sem o projeto, esses comandos também usam a pasta atual ou o seletor. Na edição,
omitir a tarefa abre uma lista pesquisável; `/` filtra as opções.

O Neovim abre um rascunho do arquivo original, posicionado na definição da
tarefa quando ela está num TOML. Tarefas em scripts abrem o script correspondente.
Use `:wq` para revisar o diff ou `:cq` para cancelar. TOML inválido oferece a
opção de voltar ao editor; o arquivo original só é substituído após a revisão.
O rascunho contém o arquivo inteiro, e o diff inclui todas as edições feitas
nele. Comentários e conteúdo não editados, além das permissões, são preservados.
Uma alteração externa durante a edição impede a gravação sobre o conteúdo novo.

Com um argumento após `konen project task edit`, ele é o nome do projeto. Para
indicar a tarefa diretamente, use os dois nomes: `konen project task edit my-app
test`. A listagem inclui tarefas locais e globais; confira o arquivo e o escopo
antes de editar uma tarefa compartilhada com outros projetos.

As edições de tarefas são salvas separadamente das abas e ações, inclusive
quando iniciadas no assistente do projeto. Nenhuma tarefa é executada, instalada
ou aprovada automaticamente pela edição. A sintaxe de scripts não é validada;
a confiança continua sob responsabilidade do mise e, para tarefas do estado,
também de `konen trust`.

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
path = "~/Projects/my-app"
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

Edite o cadastro pelo assistente com `konen project edit [NOME]`. `show`, `list`
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
