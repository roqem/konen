package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const stateTrustVersion = 1

// ExecutableSurface is the part of a state repository that can execute code
// automatically or place commands on the user's PATH.
var ExecutableSurface = []string{
	"mise.toml",
	"mise-tasks",
	".mise-tasks",
	filepath.Join("mise", "tasks"),
	filepath.Join(".mise", "tasks"),
	filepath.Join(".config", "mise", "tasks"),
	filepath.Join("scripts", "bin"),
}

type TrustStore struct {
	Path string
}

type stateTrustFile struct {
	Version int                 `toml:"version"`
	States  []trustedStateEntry `toml:"states"`
}

type trustedStateEntry struct {
	Path   string `toml:"path"`
	SHA256 string `toml:"sha256"`
}

func (s TrustStore) IsTrusted(stateDir string) (bool, error) {
	file, err := s.load()
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	digest, canonical, _, err := ExecutionDigest(stateDir)
	if err != nil {
		return false, err
	}
	for _, entry := range file.States {
		if entry.Path == canonical {
			return entry.SHA256 == digest, nil
		}
	}
	return false, nil
}

func (s TrustStore) Trust(stateDir string) ([]string, error) {
	digest, canonical, files, err := ExecutionDigest(stateDir)
	if err != nil {
		return nil, err
	}
	file, err := s.load()
	if errors.Is(err, fs.ErrNotExist) {
		file = stateTrustFile{Version: stateTrustVersion}
	} else if err != nil {
		return nil, err
	}

	found := false
	for index := range file.States {
		if file.States[index].Path == canonical {
			file.States[index].SHA256 = digest
			found = true
			break
		}
	}
	if !found {
		file.States = append(file.States, trustedStateEntry{Path: canonical, SHA256: digest})
	}
	sort.Slice(file.States, func(i, j int) bool { return file.States[i].Path < file.States[j].Path })
	if err := s.save(file); err != nil {
		return nil, err
	}
	return files, nil
}

func ExecutionDigest(stateDir string) (string, string, []string, error) {
	canonical, err := filepath.Abs(stateDir)
	if err != nil {
		return "", "", nil, err
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return "", "", nil, err
	}

	files, err := executionFiles(canonical)
	if err != nil {
		return "", "", nil, err
	}
	digest := sha256.New()
	for _, relative := range files {
		path := filepath.Join(canonical, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return "", "", nil, err
		}
		if !info.Mode().IsRegular() {
			return "", "", nil, fmt.Errorf("a superfície executável não aceita links ou arquivos especiais: %s", path)
		}
		if err := hashFile(digest, relative, info.Mode().Perm(), path); err != nil {
			return "", "", nil, err
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), canonical, files, nil
}

func executionFiles(stateDir string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string
	for _, relativeRoot := range ExecutableSurface {
		root := filepath.Join(stateDir, relativeRoot)
		info, err := os.Lstat(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("a superfície executável não aceita diretórios simbólicos: %s", root)
		}
		if !info.IsDir() {
			relative := filepath.ToSlash(relativeRoot)
			if !seen[relative] {
				files = append(files, relative)
				seen[relative] = true
			}
			continue
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == root || entry.IsDir() {
				return nil
			}
			if entry.Name() == ".gitkeep" {
				return nil
			}
			relative, err := filepath.Rel(stateDir, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if !seen[relative] {
				files = append(files, relative)
				seen[relative] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func hashFile(digest hash.Hash, relative string, mode fs.FileMode, path string) error {
	if _, err := io.WriteString(digest, fmt.Sprintf("%d:%s:%04o\x00", len(relative), relative, mode)); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(digest, file)
	return err
}

func (s TrustStore) load() (stateTrustFile, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return stateTrustFile{}, err
	}
	var file stateTrustFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return stateTrustFile{}, fmt.Errorf("aprovações de estado inválidas em %s: %w", s.Path, err)
	}
	if file.Version != stateTrustVersion {
		return stateTrustFile{}, fmt.Errorf("versão de aprovações de estado não suportada: %d", file.Version)
	}
	return file, nil
}

func (s TrustStore) save(file stateTrustFile) error {
	file.Version = stateTrustVersion
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".state-trust-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, s.Path)
}

func IsPersonalCommand(relative string) bool {
	return strings.HasPrefix(filepath.ToSlash(relative), "scripts/bin/")
}

func TaskName(relative string) (string, bool) {
	portable := filepath.ToSlash(relative)
	for _, root := range ExecutableSurface[1 : len(ExecutableSurface)-1] {
		prefix := strings.TrimSuffix(filepath.ToSlash(root), "/") + "/"
		if !strings.HasPrefix(portable, prefix) {
			continue
		}
		name := strings.TrimPrefix(portable, prefix)
		if name == "" || strings.HasPrefix(filepath.Base(name), ".") {
			return "", false
		}
		name = strings.TrimSuffix(name, "/_default")
		return strings.ReplaceAll(name, "/", ":"), true
	}
	return "", false
}
