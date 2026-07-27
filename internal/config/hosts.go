package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SavedHost is a user-defined Docker endpoint (favorites).
type SavedHost struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Note string `json:"note,omitempty"`
}

type hostsFile struct {
	Hosts []SavedHost `json:"hosts"`
}

// Dir returns ~/.config/dockafe (or $XDG_CONFIG_HOME/dockafe).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "dockafe"), nil
}

// HostsPath is the JSON file for saved Docker hosts.
func HostsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hosts.json"), nil
}

// LoadHosts reads saved hosts. Missing file → empty list.
func LoadHosts() ([]SavedHost, error) {
	path, err := HostsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f hostsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make([]SavedHost, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		h.Name = strings.TrimSpace(h.Name)
		h.Host = strings.TrimSpace(h.Host)
		h.Note = strings.TrimSpace(h.Note)
		if h.Name == "" || h.Host == "" {
			continue
		}
		out = append(out, h)
	}
	return out, nil
}

// SaveHosts writes the full favorites list atomically.
func SaveHosts(hosts []SavedHost) error {
	path, err := HostsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	clean := make([]SavedHost, 0, len(hosts))
	for _, h := range hosts {
		h.Name = strings.TrimSpace(h.Name)
		h.Host = strings.TrimSpace(h.Host)
		h.Note = strings.TrimSpace(h.Note)
		if h.Name == "" || h.Host == "" {
			continue
		}
		clean = append(clean, h)
	}
	b, err := json.MarshalIndent(hostsFile{Hosts: clean}, "", "  ")
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

// UpsertHost adds or updates a host matched by URL (case-insensitive scheme+rest trim).
func UpsertHost(name, hostURL, note string) error {
	name = strings.TrimSpace(name)
	hostURL = strings.TrimSpace(hostURL)
	note = strings.TrimSpace(note)
	if name == "" {
		return fmt.Errorf("name required")
	}
	if hostURL == "" {
		return fmt.Errorf("host URL required")
	}
	list, err := LoadHosts()
	if err != nil {
		return err
	}
	found := false
	for i := range list {
		if strings.EqualFold(list[i].Host, hostURL) {
			list[i].Name = name
			if note != "" {
				list[i].Note = note
			}
			found = true
			break
		}
	}
	if !found {
		list = append(list, SavedHost{Name: name, Host: hostURL, Note: note})
	}
	return SaveHosts(list)
}

// RemoveHost deletes a saved host by exact URL match.
func RemoveHost(hostURL string) error {
	hostURL = strings.TrimSpace(hostURL)
	if hostURL == "" {
		return fmt.Errorf("empty host")
	}
	list, err := LoadHosts()
	if err != nil {
		return err
	}
	out := make([]SavedHost, 0, len(list))
	removed := false
	for _, h := range list {
		if strings.EqualFold(h.Host, hostURL) {
			removed = true
			continue
		}
		out = append(out, h)
	}
	if !removed {
		return fmt.Errorf("saved host not found")
	}
	return SaveHosts(out)
}
