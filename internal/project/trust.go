package project

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

const trustVersion = 1

type TrustStore struct {
	Path string
}

type trustFile struct {
	Version  int            `toml:"version"`
	Projects []trustedEntry `toml:"projects"`
}

type trustedEntry struct {
	Path   string `toml:"path"`
	SHA256 string `toml:"sha256"`
}

func (s TrustStore) IsTrusted(manifestPath string) (bool, error) {
	file, err := s.load()
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	digest, canonical, err := manifestDigest(manifestPath)
	if err != nil {
		return false, err
	}
	for _, entry := range file.Projects {
		if entry.Path == canonical {
			return entry.SHA256 == digest, nil
		}
	}
	return false, nil
}

func (s TrustStore) Trust(manifestPath string) error {
	digest, canonical, err := manifestDigest(manifestPath)
	if err != nil {
		return err
	}
	file, err := s.load()
	if errors.Is(err, fs.ErrNotExist) {
		file = trustFile{Version: trustVersion}
	} else if err != nil {
		return err
	}

	found := false
	for index := range file.Projects {
		if file.Projects[index].Path == canonical {
			file.Projects[index].SHA256 = digest
			found = true
			break
		}
	}
	if !found {
		file.Projects = append(file.Projects, trustedEntry{Path: canonical, SHA256: digest})
	}
	sort.Slice(file.Projects, func(i, j int) bool { return file.Projects[i].Path < file.Projects[j].Path })
	return s.save(file)
}

func (s TrustStore) load() (trustFile, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return trustFile{}, err
	}
	var file trustFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return trustFile{}, fmt.Errorf("aprovações de projeto inválidas em %s: %w", s.Path, err)
	}
	if file.Version != trustVersion {
		return trustFile{}, fmt.Errorf("versão de aprovações não suportada: %d", file.Version)
	}
	return file, nil
}

func (s TrustStore) save(file trustFile) error {
	file.Version = trustVersion
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".projects-trust-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.Path)
}

func manifestDigest(path string) (string, string, error) {
	canonical, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(canonical)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), canonical, nil
}
