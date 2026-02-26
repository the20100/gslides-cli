package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// AuthMethod identifies which authentication type is configured.
const (
	AuthMethodServiceAccount = "service_account"
	AuthMethodOAuth2         = "oauth2"
)

// Config holds the persisted user configuration.
// Supports two auth methods:
//   - service_account: a service account JSON file (recommended for automation)
//   - oauth2:          OAuth2 desktop app flow (for interactive use)
type Config struct {
	AuthMethod string `json:"auth_method"` // "service_account" or "oauth2"

	// Service account fields.
	ServiceAccountJSON string `json:"service_account_json,omitempty"`

	// OAuth2 fields.
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "g-slides", "config.json"), nil
}

// Load reads the config file. Returns an empty Config (not an error) if the file doesn't exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config file with 0600 permissions.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Clear removes the config file (logout).
func Clear() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Path returns the config file path for display purposes.
func Path() string {
	p, _ := configPath()
	return p
}
