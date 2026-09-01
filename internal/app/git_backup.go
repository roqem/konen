package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/roqem/konen/internal/ui"
)

type gitBackupStatus struct {
	repository   bool
	hasCommit    bool
	dirty        bool
	branch       string
	remotes      []string
	inspectError error
}

func (a *App) inspectGitBackup(ctx context.Context, stateDir string) gitBackupStatus {
	status := gitBackupStatus{branch: "main"}
	gitMetadata, err := os.Lstat(filepath.Join(stateDir, ".git"))
	if errors.Is(err, fs.ErrNotExist) {
		return status
	} else if err != nil {
		status.inspectError = err
		return status
	}
	status.repository = true
	if gitMetadata.Mode()&os.ModeSymlink != 0 || !gitMetadata.IsDir() {
		status.inspectError = errors.New("a pasta .git precisa ser um diretório real para ser inspecionada")
		return status
	}

	gitPath, err := a.options.Runner.LookPath("git")
	if err != nil {
		status.inspectError = errors.New("git não está instalado")
		return status
	}
	inside, err := a.options.Runner.Output(ctx, stateDir, gitPath, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		status.inspectError = errors.New("a pasta .git existe, mas o repositório não pôde ser consultado")
		return status
	}

	if branch, branchErr := a.options.Runner.Output(ctx, stateDir, gitPath, "branch", "--show-current"); branchErr == nil {
		if branch = strings.TrimSpace(branch); branch != "" {
			status.branch = branch
		}
	}
	if _, headErr := a.options.Runner.Output(ctx, stateDir, gitPath, "rev-parse", "--verify", "--quiet", "HEAD"); headErr == nil {
		status.hasCommit = true
	}
	changes, err := a.options.Runner.OutputEnv(
		ctx, stateDir, []string{"GIT_OPTIONAL_LOCKS=0"}, gitPath,
		"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"status", "--porcelain=v1", "--untracked-files=all",
	)
	if err != nil {
		status.inspectError = errors.New("não foi possível consultar as mudanças locais do estado")
		return status
	}
	status.dirty = strings.TrimSpace(changes) != ""

	remoteOutput, err := a.options.Runner.Output(ctx, stateDir, gitPath, "remote")
	if err != nil {
		status.inspectError = errors.New("não foi possível consultar os remotos do estado")
		return status
	}
	for _, remote := range strings.Fields(remoteOutput) {
		if remote != "" {
			status.remotes = append(status.remotes, remote)
		}
	}
	sort.Strings(status.remotes)
	return status
}

func renderGitBackup(status gitBackupStatus, stateDir string) string {
	var rows [][]string
	switch {
	case status.inspectError != nil:
		rows = append(rows, []string{"Repositório", "indisponível", status.inspectError.Error()})
	case !status.repository:
		rows = append(rows,
			[]string{"Repositório", "não iniciado", "inicialize o Git antes do primeiro backup"},
			[]string{"Primeiro commit", "pendente", "revise os arquivos antes de versionar"},
			[]string{"Remoto", "ausente", "prefira um repositório privado"},
		)
	default:
		commitState := "criado"
		commitGuidance := "histórico iniciado"
		if !status.hasCommit {
			commitState = "pendente"
			commitGuidance = "revise os arquivos antes de versionar"
		}
		changeState := "nenhuma"
		changeGuidance := "árvore de trabalho limpa"
		if status.dirty {
			changeState = "presentes"
			changeGuidance = "ainda não foram salvas num commit"
		}
		remoteState := strings.Join(status.remotes, ", ")
		remoteGuidance := "configurado"
		if remoteState == "" {
			remoteState = "ausente"
			remoteGuidance = "adicione um remoto privado para restaurar em outro PC"
		}
		rows = append(rows,
			[]string{"Primeiro commit", commitState, commitGuidance},
			[]string{"Mudanças locais", changeState, changeGuidance},
			[]string{"Remoto", remoteState, remoteGuidance},
		)
	}

	var output strings.Builder
	output.WriteByte('\n')
	output.WriteString(ui.RenderTable([]string{"Backup Git", "Estado", "Orientação"}, rows))
	if status.inspectError != nil {
		return output.String()
	}

	quotedState := shellQuote(stateDir)
	showedCommands := false
	if !status.repository || !status.hasCommit {
		showedCommands = true
		output.WriteString("\nPrimeiro backup, quando o conteúdo estiver correto:\n")
		if !status.repository {
			fmt.Fprintf(&output, "  git -C %s init --initial-branch=main\n", quotedState)
		}
		fmt.Fprintf(&output, "  git -C %s status\n", quotedState)
		fmt.Fprintf(&output, "  git -C %s diff\n", quotedState)
		fmt.Fprintf(&output, "  git -C %s add .\n", quotedState)
		fmt.Fprintf(&output, "  git -C %s diff --cached\n", quotedState)
		fmt.Fprintf(&output, "  git -C %s commit -m 'configura meu ambiente'\n", quotedState)
	} else if status.dirty {
		showedCommands = true
		output.WriteString("\nPara revisar as mudanças locais:\n")
		fmt.Fprintf(&output, "  git -C %s status\n", quotedState)
		fmt.Fprintf(&output, "  git -C %s diff\n", quotedState)
	}
	if len(status.remotes) == 0 {
		showedCommands = true
		repositoryName := shellQuote(filepath.Base(stateDir))
		output.WriteString("\nPara poder restaurar em outro PC, conecte um remoto privado depois do commit:\n")
		fmt.Fprintf(
			&output, "  gh repo create %s --private --source=%s --remote=origin --push\n",
			repositoryName, quotedState,
		)
		output.WriteString("Ou crie o repositório pelo provedor e use:\n")
		fmt.Fprintf(&output, "  git -C %s remote add origin URL_DO_REPOSITORIO\n", quotedState)
		fmt.Fprintf(&output, "  git -C %s push -u origin %s\n", quotedState, shellQuote(status.branch))
	}
	if showedCommands {
		output.WriteString("O Konen apenas inspeciona e orienta; nenhum desses comandos foi executado.\n")
	}
	return output.String()
}
