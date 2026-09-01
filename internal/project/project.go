package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
	"github.com/roqem/konen/internal/compat"
)

const manifestVersion = 1

type Manifest struct {
	Version         int    `toml:"version"`
	Path            string `toml:"path"`
	Shell           string `toml:"shell,omitempty"`
	KeepInvokingTab *bool  `toml:"keep_invoking_tab,omitempty"`
	Tabs            []Tab  `toml:"tabs"`
}

type Tab struct {
	Title   string `toml:"title"`
	Command string `toml:"command,omitempty"`
	Hold    bool   `toml:"hold,omitempty"`
}

type Named struct {
	Name     string
	Path     string
	Manifest Manifest
}

type Migration struct {
	FromVersion int
	ToVersion   int
	Before      []byte
	After       []byte
	Manifest    Manifest
}

func (m Migration) Needed() bool {
	return m.FromVersion != m.ToVersion
}

type Store struct {
	StateDir string
	HomeDir  string
}

func (m Manifest) KeepsInvokingTab() bool {
	return m.KeepInvokingTab == nil || *m.KeepInvokingTab
}

func (s Store) List() ([]Named, error) {
	dir := filepath.Join(s.StateDir, "projects")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	projects := make([]Named, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".toml")
		manifest, path, err := s.Load(name)
		if err != nil {
			return nil, err
		}
		projects = append(projects, Named{Name: name, Path: path, Manifest: manifest})
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

func (s Store) Load(name string) (Manifest, string, error) {
	if err := ValidateName(name); err != nil {
		return Manifest{}, "", err
	}
	path := s.ManifestPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, path, err
	}

	manifest, err := Decode(data, path)
	return manifest, path, err
}

func Decode(data []byte, path string) (Manifest, error) {
	version, _, err := compat.TOMLVersion(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("projeto inválido em %s: %w", path, err)
	}
	if version != manifestVersion {
		return Manifest{}, fmt.Errorf("projeto incompatível em %s: %w", path, compat.VersionError{
			Format: "o manifesto de projeto", Found: version, Current: manifestVersion,
			Migratable: version == 0,
		})
	}
	return decodeCurrent(data, path)
}

func PlanMigration(path string) (Migration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Migration{}, err
	}
	version, present, err := compat.TOMLVersion(data)
	if err != nil {
		return Migration{}, fmt.Errorf("projeto inválido em %s: %w", path, err)
	}
	if version < 0 || version > manifestVersion {
		return Migration{}, fmt.Errorf("projeto incompatível em %s: %w", path, compat.VersionError{Format: "o manifesto de projeto", Found: version, Current: manifestVersion})
	}
	after := append([]byte(nil), data...)
	for next := version; next < manifestVersion; next++ {
		switch next {
		case 0:
			after, err = migrateZeroToOne(after, present)
		default:
			return Migration{}, fmt.Errorf("projeto incompatível em %s: %w", path, compat.VersionError{Format: "o manifesto de projeto", Found: next, Current: manifestVersion})
		}
		if err != nil {
			return Migration{}, fmt.Errorf("não foi possível migrar o projeto em %s: %w", path, err)
		}
		present = true
	}
	manifest, err := decodeCurrent(after, path)
	if err != nil {
		return Migration{}, err
	}
	return Migration{
		FromVersion: version, ToVersion: manifestVersion,
		Before: append([]byte(nil), data...), After: after, Manifest: manifest,
	}, nil
}

func decodeCurrent(data []byte, path string) (Manifest, error) {
	var manifest Manifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("projeto inválido em %s: %w", path, err)
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, fmt.Errorf("projeto inválido em %s: %w", path, err)
	}
	return manifest, nil
}

func migrateZeroToOne(data []byte, versionPresent bool) ([]byte, error) {
	if !versionPresent {
		return append([]byte("version = 1\n"), data...), nil
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	document["version"] = int64(1)
	updated, err := toml.Marshal(document)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s Store) Save(name string, manifest Manifest) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	manifest.Version = manifestVersion
	if err := Validate(manifest); err != nil {
		return "", err
	}

	data, err := toml.Marshal(manifest)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.StateDir, "projects")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := s.ManifestPath(name)
	tmp, err := os.CreateTemp(dir, ".project-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s Store) ManifestPath(name string) string {
	return filepath.Join(s.StateDir, "projects", name+".toml")
}

func (s Store) ResolveProjectPath(manifest Manifest) (string, error) {
	return ResolvePath(manifest.Path, s.HomeDir)
}

func (s Store) MatchDirectory(dir string) ([]Named, error) {
	projects, err := s.List()
	if err != nil {
		return nil, err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	matches := make([]Named, 0)
	for _, candidate := range projects {
		root, err := s.ResolveProjectPath(candidate.Manifest)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(root, absDir)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			matches = append(matches, candidate)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		left, _ := s.ResolveProjectPath(matches[i].Manifest)
		right, _ := s.ResolveProjectPath(matches[j].Manifest)
		return len(left) > len(right)
	})
	return matches, nil
}

func Validate(manifest Manifest) error {
	if manifest.Version != manifestVersion {
		return fmt.Errorf("versão de projeto não suportada: %d", manifest.Version)
	}
	if strings.TrimSpace(manifest.Path) == "" {
		return errors.New("path não pode ser vazio")
	}
	if containsLineBreak(manifest.Path) || containsLineBreak(manifest.Shell) {
		return errors.New("path e shell devem ocupar uma única linha")
	}
	if len(manifest.Tabs) == 0 {
		return errors.New("defina ao menos uma aba")
	}
	seen := make(map[string]bool, len(manifest.Tabs))
	for index, tab := range manifest.Tabs {
		title := strings.TrimSpace(tab.Title)
		if title == "" {
			return fmt.Errorf("tabs[%d].title não pode ser vazio", index)
		}
		if containsLineBreak(tab.Title) || containsLineBreak(tab.Command) {
			return fmt.Errorf("tabs[%d] deve ocupar uma única linha", index)
		}
		key := strings.ToLower(title)
		if seen[key] {
			return fmt.Errorf("título de aba repetido: %s", title)
		}
		seen[key] = true
	}
	return nil
}

func containsLineBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}

func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." {
		return errors.New("o nome do projeto não pode ser vazio")
	}
	for _, character := range name {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("._-", character) {
			continue
		}
		return fmt.Errorf("nome de projeto inválido: %q", name)
	}
	return nil
}

func ResolvePath(input, homeDir string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", errors.New("o caminho do projeto não pode ser vazio")
	}
	if input == "~" {
		input = homeDir
	} else if strings.HasPrefix(input, "~/") {
		input = filepath.Join(homeDir, strings.TrimPrefix(input, "~/"))
	}
	return filepath.Abs(input)
}

func PortablePath(input, homeDir string) (string, error) {
	resolved, err := ResolvePath(input, homeDir)
	if err != nil {
		return "", err
	}
	home, err := filepath.Abs(homeDir)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(home, resolved)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if relative == "." {
			return "~", nil
		}
		return filepath.ToSlash(filepath.Join("~", relative)), nil
	}
	return resolved, nil
}
