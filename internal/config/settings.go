package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings holds persistent app preferences.
type Settings struct {
	// RemoteReadOnly blocks mutating Docker API calls on remote daemons.
	// Default true when the field is omitted from disk.
	RemoteReadOnly *bool `json:"remote_read_only"`
}

// DefaultSettings returns safe defaults.
func DefaultSettings() Settings {
	v := true
	return Settings{RemoteReadOnly: &v}
}

// SettingsPath is ~/.config/dockafe/settings.json.
func SettingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// LoadSettings reads settings.json. Missing file → defaults.
func LoadSettings() (Settings, error) {
	def := DefaultSettings()
	path, err := SettingsPath()
	if err != nil {
		return def, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return def, nil
		}
		return def, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return def, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.RemoteReadOnly == nil {
		s.RemoteReadOnly = def.RemoteReadOnly
	}
	return s, nil
}

// SaveSettings writes settings atomically.
func SaveSettings(s Settings) error {
	if s.RemoteReadOnly == nil {
		v := true
		s.RemoteReadOnly = &v
	}
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// IsRemoteReadOnly reports the effective remote write lock.
func (s Settings) IsRemoteReadOnly() bool {
	if s.RemoteReadOnly == nil {
		return true
	}
	return *s.RemoteReadOnly
}

// SetRemoteReadOnly updates and persists the remote write lock.
func SetRemoteReadOnly(locked bool) (Settings, error) {
	s, err := LoadSettings()
	if err != nil {
		return s, err
	}
	s.RemoteReadOnly = &locked
	if err := SaveSettings(s); err != nil {
		return s, err
	}
	return s, nil
}
