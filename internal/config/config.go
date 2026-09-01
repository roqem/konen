package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/roqem/konen/internal/compat"
)

const currentVersion = 1

type Config struct {
	Version  int    `toml:"version"`
	StateDir string `toml:"state_dir"`
}

type Migration struct {
	FromVersion int
	ToVersion   int
	Before      []byte
	After       []byte
	Config      Config
}

func (m Migration) Needed() bool {
	return m.FromVersion != m.ToVersion
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
	return Decode(data, path)
}

func Decode(data []byte, path string) (Config, error) {
	version, _, err := compat.TOMLVersion(data)
	if err != nil {
		return Config{}, fmt.Errorf("configuração inválida em %s: %w", path, err)
	}
	if version != currentVersion {
		return Config{}, fmt.Errorf("configuração incompatível em %s: %w", path, compat.VersionError{
			Format: "a configuração local", Found: version, Current: currentVersion,
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
		return Migration{}, fmt.Errorf("configuração inválida em %s: %w", path, err)
	}
	if version < 0 || version > currentVersion {
		return Migration{}, fmt.Errorf("configuração incompatível em %s: %w", path, compat.VersionError{Format: "a configuração local", Found: version, Current: currentVersion})
	}
	after := append([]byte(nil), data...)
	for next := version; next < currentVersion; next++ {
		switch next {
		case 0:
			after, err = migrateZeroToOne(after, present)
		default:
			return Migration{}, fmt.Errorf("configuração incompatível em %s: %w", path, compat.VersionError{Format: "a configuração local", Found: next, Current: currentVersion})
		}
		if err != nil {
			return Migration{}, fmt.Errorf("não foi possível migrar a configuração em %s: %w", path, err)
		}
		present = true
	}
	cfg, err := decodeCurrent(after, path)
	if err != nil {
		return Migration{}, err
	}
	return Migration{
		FromVersion: version, ToVersion: currentVersion,
		Before: append([]byte(nil), data...), After: after, Config: cfg,
	}, nil
}

func decodeCurrent(data []byte, path string) (Config, error) {
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("configuração inválida em %s: %w", path, err)
	}
	if cfg.StateDir == "" {
		return Config{}, errors.New("state_dir não foi definido")
	}
	return cfg, nil
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
