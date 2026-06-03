// Package config models darwinvpn's YAML profile file (docs/pj.md §8).
//
// Phase 1 covers Mode A only: a profile points at an already-installed
// system VPN by display_name or UUID. mobileconfig generation, Keychain /
// 1Password integration, and Connect On Demand wiring land in Phase 2.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the supported schema version. Files with a newer or
// missing version are accepted with a soft warning at the caller's
// discretion; this package only rejects values older than 1.
const CurrentVersion = 1

// Config mirrors the YAML root described in docs/pj.md §8.
type Config struct {
	Version  int       `yaml:"version"`
	Default  string    `yaml:"default,omitempty"`
	Profiles []Profile `yaml:"profiles"`
}

type Profile struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description,omitempty"`
	System      System  `yaml:"system"`
	IKEv2       *IKEv2  `yaml:"ikev2,omitempty"`
	Secret      *Secret `yaml:"secret,omitempty"`
}

// System holds the link to an already-installed macOS VPN configuration.
// Either UUID (preferred) or DisplayName is required so darwinvpn can match
// against the live NEConfiguration list.
type System struct {
	DisplayName string `yaml:"display_name,omitempty"`
	UUID        string `yaml:"uuid,omitempty"`
}

type IKEv2 struct {
	Server   string    `yaml:"server"`
	RemoteID string    `yaml:"remote_id,omitempty"`
	LocalID  string    `yaml:"local_id,omitempty"`
	Auth     string    `yaml:"auth,omitempty"` // eap | certificate | shared-secret
	OnDemand *OnDemand `yaml:"on_demand,omitempty"`
}

type OnDemand struct {
	Enabled      bool     `yaml:"enabled"`
	MatchDomains []string `yaml:"match_domains,omitempty"`
}

type Secret struct {
	Provider string `yaml:"provider"` // keychain | 1password
	Ref      string `yaml:"ref"`
}

// ErrNotFound is returned by Load when the config file does not exist on
// disk. Callers typically downgrade this to a no-op (the CLI continues to
// resolve services by their system display_name).
var ErrNotFound = errors.New("config: file does not exist")

// Load reads and parses the YAML at path. A leading "~" or "~/" is expanded
// against the current user's home directory.
func Load(path string) (*Config, error) {
	expanded, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(expanded)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("config: read %s: %w", expanded, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", expanded, err)
	}
	if c.Version != 0 && c.Version < CurrentVersion {
		return nil, fmt.Errorf("config: schema version %d is older than supported %d",
			c.Version, CurrentVersion)
	}
	return &c, nil
}

// Save serializes c to path, creating parent directories as needed and
// applying 0600 permissions because profiles may eventually reference
// secret materials.
func Save(c *Config, path string) error {
	expanded, err := ExpandPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o700); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", filepath.Dir(expanded), err)
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(expanded, out, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", expanded, err)
	}
	return nil
}

// ExpandPath resolves a leading "~/" to the user's home directory.
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("config: empty path")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: resolve home: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// DefaultPath returns the canonical config location
// (~/.config/darwinvpn/config.yaml).
func DefaultPath() string {
	return "~/.config/darwinvpn/config.yaml"
}

// ProfileByName returns the profile whose Name matches case-insensitively.
// When name is empty and Default is set, the default profile is returned.
// When name is empty and Default is empty, the lone profile is returned;
// if more than one is registered, the caller must disambiguate.
func (c *Config) ProfileByName(name string) (*Profile, error) {
	if c == nil {
		return nil, errors.New("config: nil")
	}
	if name == "" {
		if c.Default != "" {
			name = c.Default
		} else if len(c.Profiles) == 1 {
			return &c.Profiles[0], nil
		} else {
			return nil, errors.New("config: multiple profiles registered; specify one")
		}
	}
	needle := strings.ToLower(name)
	for i := range c.Profiles {
		if strings.ToLower(c.Profiles[i].Name) == needle {
			return &c.Profiles[i], nil
		}
	}
	return nil, fmt.Errorf("config: profile %q not found", name)
}
