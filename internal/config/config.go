package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const currentVersion = 1

type Config struct {
	Version  int    `toml:"version"`
	StateDir string `toml:"state_dir"`
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "konen", "config.toml"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("configuração inválida em %s: %w", path, err)
	}
	if cfg.Version != currentVersion {
		return Config{}, fmt.Errorf("versão de configuração não suportada: %d", cfg.Version)
	}
	if cfg.StateDir == "" {
		return Config{}, errors.New("state_dir não foi definido")
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if cfg.StateDir == "" {
		return errors.New("state_dir não pode ser vazio")
	}
	cfg.Version = currentVersion

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
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
	return os.Rename(tmpPath, path)
}
