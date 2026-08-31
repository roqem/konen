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
  local -a commands project_actions
  commands=(
    'init:configura ou cria o estado'
    'status:mostra tudo que o estado declara'
    'plan:mostra exatamente o que mudaria'
    'diff:mostra diferenças dos dotfiles'
    'apply:aplica o estado com mise'
    'add:adiciona um arquivo ou diretório'
    'project:gerencia projetos e suas sessões'
    'dev:abre um projeto no Kitty'
    'trust:confia no estado após revisão'
    'doctor:diagnostica a instalação'
    'completion:gera autocomplete para o shell'
    'version:mostra a versão'
    'help:mostra ajuda'
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
        '--from=[clona um repositório Git]:URL do repositório:_urls' \
        '1:pasta do estado:_directories'
      ;;
    status|plan|diff|trust|doctor|version|help)
      _arguments
      ;;
    apply)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--yes[não pede confirmação]' \
        '--dry-run[mostra o plano sem alterar a máquina]'
      ;;
    add)
      _arguments \
        '(-h --help)'{-h,--help}'[mostra ajuda]' \
        '--mode=[define o modo do dotfile]:modo:(symlink copy template)' \
        '*:arquivo ou diretório:_files'
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
    COMPREPLY=( $(compgen -W 'init status plan diff apply add project dev trust doctor completion version help' -- "$current") )
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
    apply)
      COMPREPLY=( $(compgen -W '--yes --dry-run -h --help' -- "$current") )
      ;;
    add)
      if [[ $previous == --mode ]]; then
        COMPREPLY=( $(compgen -W 'symlink copy template' -- "$current") )
      elif [[ $current == -* ]]; then
        COMPREPLY=( $(compgen -W '--mode -h --help' -- "$current") )
      else
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
complete -c konen -n '__fish_use_subcommand' -a add -d 'Adiciona um arquivo ou diretório'
complete -c konen -n '__fish_use_subcommand' -a project -d 'Gerencia projetos e suas sessões'
complete -c konen -n '__fish_use_subcommand' -a dev -d 'Abre um projeto no Kitty'
complete -c konen -n '__fish_use_subcommand' -a trust -d 'Confia no estado após revisão'
complete -c konen -n '__fish_use_subcommand' -a doctor -d 'Diagnostica a instalação'
complete -c konen -n '__fish_use_subcommand' -a completion -d 'Gera autocomplete para o shell'
complete -c konen -n '__fish_use_subcommand' -a version -d 'Mostra a versão'
complete -c konen -n '__fish_use_subcommand' -a help -d 'Mostra ajuda'
complete -c konen -n '__fish_seen_subcommand_from init' -l git -d 'Inicializa um repositório Git'
complete -c konen -n '__fish_seen_subcommand_from init' -l from -r -d 'Clona um repositório Git'
complete -c konen -n '__fish_seen_subcommand_from apply' -l yes -d 'Não pede confirmação'
complete -c konen -n '__fish_seen_subcommand_from apply' -l dry-run -d 'Mostra o plano sem alterar a máquina'
complete -c konen -n '__fish_seen_subcommand_from add' -l mode -r -a 'symlink copy template' -d 'Modo do dotfile'
complete -c konen -n '__fish_seen_subcommand_from dev' -l dry-run -d 'Mostra a sessão sem abrir abas'
complete -c konen -n '__fish_seen_subcommand_from dev' -a '(konen __complete projects 2>/dev/null)' -d 'Projeto'
complete -c konen -n '__fish_seen_subcommand_from project' -a 'add edit list show trust'
complete -c konen -n '__fish_seen_subcommand_from completion' -a 'zsh bash fish'
`
