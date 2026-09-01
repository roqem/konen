package app

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/roqem/konen/internal/config"
	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

type migrationItem struct {
	kind           string
	displayPath    string
	path           string
	backupRelative string
	projectName    string
	fromVersion    int
	toVersion      int
	before         []byte
	after          []byte
	mode           fs.FileMode
	info           fs.FileInfo
}

func (i migrationItem) needed() bool {
	return i.fromVersion != i.toVersion
}

type migrationPlan struct {
	items []migrationItem
}

func (p migrationPlan) pending() []migrationItem {
	var pending []migrationItem
	for _, item := range p.items {
		if item.needed() {
			pending = append(pending, item)
		}
	}
	return pending
}

func (a *App) runMigrate(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	dryRun := flags.Bool("dry-run", false, "mostra migrações sem alterar arquivos")
	yes := flags.Bool("yes", false, "migra sem pedir confirmação")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("migrate não aceita argumentos posicionais")
	}
	if *dryRun && *yes {
		return errors.New("use apenas --dry-run ou --yes")
	}

	plan, err := a.buildMigrationPlan()
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("Konen ainda não foi configurado; execute `konen init`")
	}
	if err != nil {
		return err
	}
	fmt.Fprint(a.options.Out, renderMigrationPlan(plan))
	for _, item := range plan.pending() {
		fmt.Fprintln(a.options.Out)
		fmt.Fprint(a.options.Out, ui.RenderDiff(item.displayPath, string(item.before), string(item.after)))
	}

	pending := plan.pending()
	if len(pending) == 0 {
		fmt.Fprintln(a.options.Out, "Nenhuma migração necessária.")
		return nil
	}
	if *dryRun {
		fmt.Fprintln(a.options.Out, "Nenhum arquivo foi alterado.")
		return nil
	}
	if !*yes {
		if !a.options.Interactive {
			return errors.New("revise com `konen migrate --dry-run` ou confirme com `konen migrate --yes`")
		}
		confirmed, err := a.options.Prompter.Confirm("Migrar estes arquivos?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(a.options.Out, "Migração cancelada.")
			return nil
		}
	}

	backupRoot, err := a.applyMigrationPlan(pending)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.options.Out, "Migração concluída: %d arquivo(s).\n", len(pending))
	fmt.Fprintf(a.options.Out, "Backup local: %s\n", displayPath(backupRoot, a.options.HomeDir))
	var projects []string
	for _, item := range pending {
		if item.projectName != "" {
			projects = append(projects, item.projectName)
		}
	}
	if len(projects) > 0 {
		fmt.Fprintln(a.options.Out, "Manifestos migrados precisam de nova aprovação antes de executar comandos:")
		for _, name := range projects {
			fmt.Fprintf(a.options.Out, "  konen project trust %s\n", name)
		}
	}
	return nil
}

func (a *App) buildMigrationPlan() (migrationPlan, error) {
	configurationInfo, err := migrationFileInfo(a.options.ConfigPath)
	if err != nil {
		return migrationPlan{}, err
	}
	configuration, err := config.PlanMigration(a.options.ConfigPath)
	if err != nil {
		return migrationPlan{}, err
	}
	plan := migrationPlan{items: []migrationItem{{
		kind: "Configuração local", displayPath: displayPath(a.options.ConfigPath, a.options.HomeDir),
		path: a.options.ConfigPath, backupRelative: "config.toml",
		fromVersion: configuration.FromVersion, toVersion: configuration.ToVersion,
		before: configuration.Before, after: configuration.After, mode: configurationInfo.Mode().Perm(), info: configurationInfo,
	}}}
	if err := state.Valid(configuration.Config.StateDir); err != nil {
		return migrationPlan{}, err
	}

	projectsDir := filepath.Join(configuration.Config.StateDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		return migrationPlan{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		if err := project.ValidateName(name); err != nil {
			return migrationPlan{}, err
		}
		path := filepath.Join(projectsDir, entry.Name())
		info, err := migrationFileInfo(path)
		if err != nil {
			return migrationPlan{}, err
		}
		projectMigration, err := project.PlanMigration(path)
		if err != nil {
			return migrationPlan{}, err
		}
		plan.items = append(plan.items, migrationItem{
			kind: "Projeto", displayPath: filepath.ToSlash(filepath.Join("projects", entry.Name())),
			path: path, backupRelative: filepath.Join("projects", entry.Name()), projectName: name,
			fromVersion: projectMigration.FromVersion, toVersion: projectMigration.ToVersion,
			before: projectMigration.Before, after: projectMigration.After, mode: info.Mode().Perm(), info: info,
		})
	}
	return plan, nil
}

func renderMigrationPlan(plan migrationPlan) string {
	rows := make([][]string, 0, len(plan.items))
	for _, item := range plan.items {
		action := "já compatível"
		if item.needed() {
			action = fmt.Sprintf("migrar v%d → v%d", item.fromVersion, item.toVersion)
		}
		rows = append(rows, []string{
			item.kind, item.displayPath,
			fmt.Sprintf("v%d", item.fromVersion), fmt.Sprintf("v%d", item.toVersion), action,
		})
	}
	return ui.RenderTable([]string{"Tipo", "Arquivo", "Encontrado", "Suportado", "Plano"}, rows)
}

func (a *App) applyMigrationPlan(items []migrationItem) (string, error) {
	for _, item := range items {
		info, err := migrationFileInfo(item.path)
		if err != nil {
			return "", err
		}
		if !os.SameFile(item.info, info) {
			return "", fmt.Errorf("%s foi substituído durante a revisão; execute `konen migrate --dry-run` novamente", item.displayPath)
		}
		current, err := os.ReadFile(item.path)
		if err != nil {
			return "", err
		}
		if !bytes.Equal(current, item.before) {
			return "", fmt.Errorf("%s mudou durante a revisão; execute `konen migrate --dry-run` novamente", item.displayPath)
		}
	}

	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	backupRoot := filepath.Join(filepath.Dir(a.options.ConfigPath), "migration-backups", timestamp)
	for _, item := range items {
		backupPath := filepath.Join(backupRoot, item.backupRelative)
		if err := writeMigrationBackup(backupPath, item.before); err != nil {
			return "", fmt.Errorf("não foi possível criar o backup de %s: %w", item.displayPath, err)
		}
	}
	for _, item := range items {
		if item.projectName == "" {
			continue
		}
		if err := a.projectTrust().Revoke(item.path); err != nil {
			return "", fmt.Errorf("não foi possível revogar a aprovação anterior de %s: %w", item.displayPath, err)
		}
	}

	var applied []migrationItem
	for _, item := range items {
		if err := writeMigrationFile(item.path, item.after, item.mode); err != nil {
			rollbackErr := rollbackMigrations(applied)
			if rollbackErr != nil {
				return "", fmt.Errorf("migração de %s falhou (%v) e o rollback também falhou (%v); restaure a partir de %s", item.displayPath, err, rollbackErr, backupRoot)
			}
			return "", fmt.Errorf("migração de %s falhou: %w; arquivos anteriores foram restaurados", item.displayPath, err)
		}
		applied = append(applied, item)
	}
	return backupRoot, nil
}

func migrationFileInfo(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("a migração aceita apenas arquivos regulares: %s", path)
	}
	return info, nil
}

func rollbackMigrations(items []migrationItem) error {
	for index := len(items) - 1; index >= 0; index-- {
		item := items[index]
		if err := writeMigrationFile(item.path, item.before, item.mode); err != nil {
			return err
		}
	}
	return nil
}

func writeMigrationBackup(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func writeMigrationFile(path string, data []byte, mode fs.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".konen-migrate-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode.Perm()); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	if directory, err := os.Open(filepath.Dir(path)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
