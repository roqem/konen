package compat

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// VersionError describes a portable Konen format that the running binary
// cannot consume without an explicit migration or update.
type VersionError struct {
	Format     string
	Found      int
	Current    int
	Migratable bool
}

func (e VersionError) Error() string {
	switch {
	case e.Found > e.Current:
		return fmt.Sprintf("%s usa a versão %d, mas este Konen suporta até %d; atualize com `konen update`", e.Format, e.Found, e.Current)
	case e.Migratable:
		return fmt.Sprintf("%s usa a versão antiga %d; revise a migração para %d com `konen migrate --dry-run`", e.Format, e.Found, e.Current)
	default:
		return fmt.Sprintf("%s usa a versão %d e não possui migração compatível para %d", e.Format, e.Found, e.Current)
	}
}

// TOMLVersion distinguishes an absent version field (the legacy version zero)
// from an explicitly declared integer.
func TOMLVersion(data []byte) (version int, present bool, err error) {
	var header struct {
		Version *int `toml:"version"`
	}
	if err := toml.Unmarshal(data, &header); err != nil {
		return 0, false, err
	}
	if header.Version == nil {
		return 0, false, nil
	}
	return *header.Version, true, nil
}
