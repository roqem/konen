package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/roqem/konen/internal/project"
)

func (a *App) runCompletion(args []string) error {
	if len(args) != 1 {
		return errors.New("uso: konen completion zsh|bash|fish")
	}
	script, err := completionScript(args[0])
	if err != nil {
		return err
	}
	_, err = io.WriteString(a.options.Out, script)
	return err
}

func (a *App) runInternalComplete(args []string) error {
	if len(args) != 1 || args[0] != "projects" {
		return errors.New("consulta de autocomplete inválida")
	}
	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	store := project.Store{StateDir: stateDir, HomeDir: a.options.HomeDir}
	projects, err := store.List()
	if err != nil {
		return err
	}
	for _, item := range projects {
		fmt.Fprintln(a.options.Out, item.Name)
	}
	return nil
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "zsh":
		return zshCompletion, nil
	case "bash":
		return bashCompletion, nil
	case "fish":
		return fishCompletion, nil
	default:
		return "", fmt.Errorf("shell não suportado: %s (use zsh, bash ou fish)", shell)
	}
}

const zshCompletion = `#compdef konen

_konen_projects() {
  local -a projects
  projects=("${(@f)$(konen __complete projects 2>/dev/null)}")
  _describe 'projeto' projects
}

_konen() {
  local -a commands tool_actions package_actions repo_actions command_actions installer_actions dotfile_actions project_actions
  commands=(
    'init:configura ou cria o estado'
    'status:mostra tudo que o estado declara'
    'plan:mostra exatamente o que mudaria'
    'diff:mostra diferenças dos dotfiles'
    'apply:aplica o estado com mise'
    'update:mostra versões e atualiza Konen e mise'
    'tool:gerencia ferramentas do estado'
    'package:gerencia pacotes do sistema no estado'
    'repo:gerencia repositórios Git no estado'
    'command:gerencia comandos pessoais no estado'
    'installer:gerencia instaladores pessoais no estado'
    'dotfile:gerencia arquivos de configuração'
    'projects:lista os projetos cadastrados'
    'project:gerencia projetos e suas sessões'
    'dev:abre um projeto no Kitty'
    'trust:confia no estado após revisão'
    'doctor:diagnostica a instalação'
    'completion:gera autocomplete para o shell'
    'version:mostra a versão'
    'help:mostra ajuda'
  )
  tool_actions=(
    'add:adiciona uma ferramenta ao estado'
  )
  package_actions=(
    'add:adiciona um pacote do sistema ao estado'
  )
  repo_actions=(
    'add:adiciona um repositório Git ao estado'
  )
  command_actions=(
    'add:cria ou importa um comando pessoal'
  )
  installer_actions=(
    'add:cria ou importa um instalador pessoal'
  )
  dotfile_actions=(
    'add:adiciona um dotfile ao estado'
  )
  project_actions=(
    'add:cadastra um projeto'
    'edit:edita um projeto'
    'list:lista os projetos'
    'show:mostra o manifesto de um projeto'
    'trust:aprova os comandos de um projeto'
  )

  if (( CURRENT == 2 )); then
    _describe 'comando' commands
    _konen_projects
    return
  fi

  local command=$words[2]
  words=($words[2,-1])
  (( CURRENT-- ))
  case $command in
    init)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--git[inicializa um repositório Git]' \
		'--from=[clona um estado; github:OWNER/REPO ativa login assistido]:origem do repositório:_urls' \
        '1:pasta do estado:_directories'
      ;;
    status)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--only=[limita a categorias separadas por vírgula]:categorias:(packages repos dotfiles tools task user)' \
        '--state=[limita a situações separadas por vírgula]:situações:(ready pending missing different unknown)'
      ;;
    diff|projects|trust|doctor|version|help)
      _arguments
      ;;
    plan)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--select[escolhe as etapas interativamente]' \
        '--only=[limita a etapas separadas por vírgula]:etapas:(packages repos dotfiles tools task)'
      ;;
    apply)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--yes[não pede confirmação]' \
        '--dry-run[mostra o plano sem alterar a máquina]' \
        '--select[escolhe as etapas interativamente]' \
        '--only=[limita a etapas separadas por vírgula]:etapas:(packages repos dotfiles tools task)'
      ;;
    update)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--dry-run[mostra versões e ações sem atualizar]' \
        '--yes[atualiza sem pedir confirmação]' \
        '--pre[inclui prereleases do Konen]' \
        '--only=[limita aos componentes separados por vírgula]:componentes:(konen mise)'
      ;;
    tool)
      if (( CURRENT == 2 )); then
        _describe 'ação' tool_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add)
          _arguments \
            '(-h --help)'{-h,--help}'[mostra ajuda]' \
            '--yes[grava sem pedir confirmação]' \
            '--dry-run[mostra a alteração sem gravar]' \
            '1:ferramenta' \
            '2:versão'
          ;;
        *) _describe 'ação' tool_actions ;;
      esac
      ;;
    package)
      if (( CURRENT == 2 )); then
        _describe 'ação' package_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add)
          _arguments \
            '(-h --help)'{-h,--help}'[mostra ajuda]' \
            '--yes[grava sem pedir confirmação]' \
            '--dry-run[mostra a alteração sem gravar]' \
            '--manager=[define o gerenciador do sistema]:gerenciador:(apt dnf pacman apk brew brew-cask flatpak flatpak-user mas)' \
            '1:pacote' \
            '2:versão'
          ;;
        *) _describe 'ação' package_actions ;;
      esac
      ;;
    repo)
      if (( CURRENT == 2 )); then
        _describe 'ação' repo_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add)
          _arguments \
            '(-h --help)'{-h,--help}'[mostra ajuda]' \
            '--yes[grava sem pedir confirmação]' \
            '--dry-run[mostra a alteração sem gravar]' \
            '1:pasta de destino:_directories' \
            '2:URL Git:_urls' \
            '3:branch, tag ou commit'
          ;;
        *) _describe 'ação' repo_actions ;;
      esac
      ;;
    command)
      if (( CURRENT == 2 )); then
        _describe 'ação' command_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add)
          _arguments \
            '(-h --help)'{-h,--help}'[mostra ajuda]' \
            '--yes[grava sem pedir confirmação]' \
            '--dry-run[mostra os arquivos sem gravar]' \
            '--from=[importa um arquivo existente]:arquivo:_files' \
            '1:nome do comando'
          ;;
        *) _describe 'ação' command_actions ;;
      esac
      ;;
    installer)
      if (( CURRENT == 2 )); then
        _describe 'ação' installer_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add)
          _arguments \
            '(-h --help)'{-h,--help}'[mostra ajuda]' \
            '--yes[grava sem pedir confirmação]' \
            '--dry-run[mostra os arquivos sem gravar]' \
            '--from=[importa um arquivo existente]:arquivo:_files' \
            '1:nome do instalador'
          ;;
        *) _describe 'ação' installer_actions ;;
      esac
      ;;
    dotfile)
      if (( CURRENT == 2 )); then
        _describe 'ação' dotfile_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add)
          _arguments \
            '(-h --help)'{-h,--help}'[mostra ajuda]' \
            '--mode=[define o modo do dotfile]:modo:(symlink copy template)' \
            '*:arquivo ou diretório:_files'
          ;;
        *) _describe 'ação' dotfile_actions ;;
      esac
      ;;
    dev)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--dry-run[mostra a sessão sem abrir abas]' \
        '1:projeto:_konen_projects'
      ;;
    completion)
      _arguments '1:shell:(zsh bash fish)'
      ;;
    project)
      if (( CURRENT == 2 )); then
        _describe 'ação' project_actions
        return
      fi
      local action=$words[2]
      words=($words[2,-1])
      (( CURRENT-- ))
      case $action in
        add) _arguments '1:pasta do projeto:_directories' ;;
        edit|show|trust) _arguments '1:projeto:_konen_projects' ;;
        list) _arguments ;;
        *) _describe 'ação' project_actions ;;
      esac
      ;;
    *)
      _arguments '--dry-run[mostra a sessão sem abrir abas]'
      ;;
  esac
}

compdef _konen konen
`

const bashCompletion = `_konen_completion() {
  local current previous command action
  COMPREPLY=()
  current="${COMP_WORDS[COMP_CWORD]}"
  previous="${COMP_WORDS[COMP_CWORD-1]}"
  command="${COMP_WORDS[1]}"

  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "init status plan diff apply update tool package repo command installer dotfile projects project dev trust doctor completion version help $(konen __complete projects 2>/dev/null)" -- "$current") )
    return
  fi

  case "$command" in
    init)
      if [[ $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--git --from -h --help' -- "$current") )
      else
        COMPREPLY=( $(compgen -d -- "$current") )
      fi
      ;;
    status)
      if [[ $previous == --only ]]; then
        COMPREPLY=( $(compgen -W 'packages repos dotfiles tools task user' -- "$current") )
      elif [[ $previous == --state ]]; then
        COMPREPLY=( $(compgen -W 'ready pending missing different unknown' -- "$current") )
      else
        COMPREPLY=( $(compgen -W '--only --state -h --help' -- "$current") )
      fi
      ;;
    plan)
      COMPREPLY=( $(compgen -W '--select --only -h --help' -- "$current") )
      ;;
    apply)
      COMPREPLY=( $(compgen -W '--yes --dry-run --select --only -h --help' -- "$current") )
      ;;
    update)
      if [[ $previous == --only ]]; then
        COMPREPLY=( $(compgen -W 'konen mise' -- "$current") )
      else
        COMPREPLY=( $(compgen -W '--dry-run --yes --pre --only -h --help' -- "$current") )
      fi
      ;;
    tool)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add' -- "$current") )
      elif [[ $action == add && $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--yes --dry-run -h --help' -- "$current") )
      fi
      ;;
    package)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add' -- "$current") )
      elif [[ $action == add && $previous == --manager ]]; then
        COMPREPLY=( $(compgen -W 'apt dnf pacman apk brew brew-cask flatpak flatpak-user mas' -- "$current") )
      elif [[ $action == add && $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--manager --yes --dry-run -h --help' -- "$current") )
      fi
      ;;
    repo)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add' -- "$current") )
      elif [[ $action == add && $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--yes --dry-run -h --help' -- "$current") )
      elif [[ $action == add && $COMP_CWORD -eq 3 ]]; then
        COMPREPLY=( $(compgen -d -- "$current") )
      fi
      ;;
    command)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add' -- "$current") )
      elif [[ $action == add && $previous == --from ]]; then
        COMPREPLY=( $(compgen -f -- "$current") )
      elif [[ $action == add && $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--from --yes --dry-run -h --help' -- "$current") )
      fi
      ;;
    installer)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add' -- "$current") )
      elif [[ $action == add && $previous == --from ]]; then
        COMPREPLY=( $(compgen -f -- "$current") )
      elif [[ $action == add && $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--from --yes --dry-run -h --help' -- "$current") )
      fi
      ;;
    dotfile)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add' -- "$current") )
      elif [[ $action == add && $previous == --mode ]]; then
        COMPREPLY=( $(compgen -W 'symlink copy template' -- "$current") )
      elif [[ $action == add && $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--mode -h --help' -- "$current") )
      elif [[ $action == add ]]; then
        COMPREPLY=( $(compgen -f -- "$current") )
      fi
      ;;
    dev)
      COMPREPLY=( $(compgen -W "--dry-run -h --help $(konen __complete projects 2>/dev/null)" -- "$current") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W 'zsh bash fish' -- "$current") )
      ;;
    project)
      action="${COMP_WORDS[2]}"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W 'add edit list show trust' -- "$current") )
      elif [[ $action == add ]]; then
        COMPREPLY=( $(compgen -d -- "$current") )
      elif [[ $action == edit || $action == show || $action == trust ]]; then
        COMPREPLY=( $(compgen -W "$(konen __complete projects 2>/dev/null)" -- "$current") )
      fi
      ;;
    *)
      COMPREPLY=( $(compgen -W '--dry-run' -- "$current") )
      ;;
  esac
}

complete -F _konen_completion konen
`

const fishCompletion = `complete -c konen -f
complete -c konen -n '__fish_use_subcommand' -a init -d 'Configura ou cria o estado'
complete -c konen -n '__fish_use_subcommand' -a status -d 'Mostra tudo que o estado declara'
complete -c konen -n '__fish_use_subcommand' -a plan -d 'Mostra exatamente o que mudaria'
complete -c konen -n '__fish_use_subcommand' -a diff -d 'Mostra diferenças dos dotfiles'
complete -c konen -n '__fish_use_subcommand' -a apply -d 'Aplica o estado com mise'
complete -c konen -n '__fish_use_subcommand' -a update -d 'Mostra versões e atualiza Konen e mise'
complete -c konen -n '__fish_use_subcommand' -a tool -d 'Gerencia ferramentas do estado'
complete -c konen -n '__fish_use_subcommand' -a package -d 'Gerencia pacotes do sistema no estado'
complete -c konen -n '__fish_use_subcommand' -a repo -d 'Gerencia repositórios Git no estado'
complete -c konen -n '__fish_use_subcommand' -a command -d 'Gerencia comandos pessoais no estado'
complete -c konen -n '__fish_use_subcommand' -a installer -d 'Gerencia instaladores pessoais no estado'
complete -c konen -n '__fish_use_subcommand' -a dotfile -d 'Gerencia arquivos de configuração'
complete -c konen -n '__fish_use_subcommand' -a projects -d 'Lista os projetos cadastrados'
complete -c konen -n '__fish_use_subcommand' -a project -d 'Gerencia projetos e suas sessões'
complete -c konen -n '__fish_use_subcommand' -a dev -d 'Abre um projeto no Kitty'
complete -c konen -n '__fish_use_subcommand' -a trust -d 'Confia no estado após revisão'
complete -c konen -n '__fish_use_subcommand' -a doctor -d 'Diagnostica a instalação'
complete -c konen -n '__fish_use_subcommand' -a completion -d 'Gera autocomplete para o shell'
complete -c konen -n '__fish_use_subcommand' -a version -d 'Mostra a versão'
complete -c konen -n '__fish_use_subcommand' -a help -d 'Mostra ajuda'
complete -c konen -n '__fish_use_subcommand' -a '(konen __complete projects 2>/dev/null)' -d 'Abre o projeto'
complete -c konen -n '__fish_seen_subcommand_from init' -l git -d 'Inicializa um repositório Git'
complete -c konen -n '__fish_seen_subcommand_from init' -l from -r -d 'Clona um estado; GitHub privado tem login assistido'
complete -c konen -n '__fish_seen_subcommand_from status' -l only -r -a 'packages repos dotfiles tools task user' -d 'Limita a categorias'
complete -c konen -n '__fish_seen_subcommand_from status' -l state -r -a 'ready pending missing different unknown' -d 'Limita a situações'
complete -c konen -n '__fish_seen_subcommand_from apply' -l yes -d 'Não pede confirmação'
complete -c konen -n '__fish_seen_subcommand_from apply' -l dry-run -d 'Mostra o plano sem alterar a máquina'
complete -c konen -n '__fish_seen_subcommand_from update' -l dry-run -d 'Mostra versões e ações sem atualizar'
complete -c konen -n '__fish_seen_subcommand_from update' -l yes -d 'Atualiza sem pedir confirmação'
complete -c konen -n '__fish_seen_subcommand_from update' -l pre -d 'Inclui prereleases do Konen'
complete -c konen -n '__fish_seen_subcommand_from update' -l only -r -a 'konen mise' -d 'Limita aos componentes'
complete -c konen -n '__fish_seen_subcommand_from plan apply' -l select -d 'Escolhe as etapas interativamente'
complete -c konen -n '__fish_seen_subcommand_from plan apply' -l only -r -a 'packages repos dotfiles tools task' -d 'Limita a etapas'
complete -c konen -n '__fish_seen_subcommand_from tool' -a add -d 'Adiciona uma ferramenta ao estado'
complete -c konen -n '__fish_seen_subcommand_from tool' -l yes -d 'Grava sem pedir confirmação'
complete -c konen -n '__fish_seen_subcommand_from tool' -l dry-run -d 'Mostra a alteração sem gravar'
complete -c konen -n '__fish_seen_subcommand_from package' -a add -d 'Adiciona um pacote do sistema ao estado'
complete -c konen -n '__fish_seen_subcommand_from package' -l manager -r -a 'apt dnf pacman apk brew brew-cask flatpak flatpak-user mas' -d 'Gerenciador do sistema'
complete -c konen -n '__fish_seen_subcommand_from package' -l yes -d 'Grava sem pedir confirmação'
complete -c konen -n '__fish_seen_subcommand_from package' -l dry-run -d 'Mostra a alteração sem gravar'
complete -c konen -n '__fish_seen_subcommand_from repo' -a add -d 'Adiciona um repositório Git ao estado'
complete -c konen -n '__fish_seen_subcommand_from repo' -l yes -d 'Grava sem pedir confirmação'
complete -c konen -n '__fish_seen_subcommand_from repo' -l dry-run -d 'Mostra a alteração sem gravar'
complete -c konen -n '__fish_seen_subcommand_from command' -a add -d 'Cria ou importa um comando pessoal'
complete -c konen -n '__fish_seen_subcommand_from command' -l from -r -F -d 'Importa um arquivo existente'
complete -c konen -n '__fish_seen_subcommand_from command' -l yes -d 'Grava sem pedir confirmação'
complete -c konen -n '__fish_seen_subcommand_from command' -l dry-run -d 'Mostra os arquivos sem gravar'
complete -c konen -n '__fish_seen_subcommand_from installer' -a add -d 'Cria ou importa um instalador pessoal'
complete -c konen -n '__fish_seen_subcommand_from installer' -l from -r -F -d 'Importa um arquivo existente'
complete -c konen -n '__fish_seen_subcommand_from installer' -l yes -d 'Grava sem pedir confirmação'
complete -c konen -n '__fish_seen_subcommand_from installer' -l dry-run -d 'Mostra os arquivos sem gravar'
complete -c konen -n '__fish_seen_subcommand_from dotfile' -a add -d 'Adiciona um dotfile ao estado'
complete -c konen -n '__fish_seen_subcommand_from dotfile' -l mode -r -a 'symlink copy template' -d 'Modo do dotfile'
complete -c konen -n '__fish_seen_subcommand_from dev' -l dry-run -d 'Mostra a sessão sem abrir abas'
complete -c konen -n '__fish_seen_subcommand_from dev' -a '(konen __complete projects 2>/dev/null)' -d 'Projeto'
complete -c konen -n '__fish_seen_subcommand_from project' -a 'add edit list show trust'
complete -c konen -n '__fish_seen_subcommand_from completion' -a 'zsh bash fish'
`
