// Package config manages the two config stores of the CLI:
//
//   - the user-level store (~/.config/itapu/config.json, mode 0600) holding
//     the account and secrets tokens — never committed;
//   - the project-level config (.itapu.json) holding only org/project/
//     environment ids, no tokens; per-developer, so `itapu init` adds it
//     to .gitignore.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DefaultBaseURL = "https://itapu.vercel.app"

// ---- user-level store ----

type UserConfig struct {
	BaseURL               string       `json:"baseUrl,omitempty"`
	AccountToken          string       `json:"accountToken,omitempty"`
	AccountTokenExpiresAt time.Time    `json:"accountTokenExpiresAt,omitempty"`
	SecretsToken          string       `json:"secretsToken,omitempty"`
	SecretsTokenExpiresAt time.Time    `json:"secretsTokenExpiresAt,omitempty"`
	SecretsTokenGrants    []TokenGrant `json:"secretsTokenGrants,omitempty"`
}

// TokenGrant records one project/environment covered by the stored secrets
// token, so `itapu init` can skip the browser approval when the requested
// grant is already held. Grant names/ids are not secrets.
type TokenGrant struct {
	OrgID           string `json:"orgId"`
	ProjectID       string `json:"projectId"`
	ProjectName     string `json:"projectName"`
	EnvironmentID   string `json:"environmentId"`
	EnvironmentSlug string `json:"environmentSlug"`
	EnvironmentName string `json:"environmentName"`
}

func userConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "itapu", "config.json"), nil
}

// LoadUser returns an empty config when the file does not exist yet.
func LoadUser() (*UserConfig, error) {
	path, err := userConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &UserConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt config at %s: %w", path, err)
	}
	return &cfg, nil
}

func SaveUser(cfg *UserConfig) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Write via a temp file so a crash never leaves a partial token file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ResolveBaseURL applies the precedence: flag > ITAPU_BASE_URL > stored > default.
func ResolveBaseURL(flagValue string, cfg *UserConfig) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("ITAPU_BASE_URL"); env != "" {
		return env
	}
	if cfg != nil && cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	return DefaultBaseURL
}

// ---- project-level config (.itapu.json) ----

const ProjectConfigName = ".itapu.json"

type ProjectGrant struct {
	ProjectID       string `json:"projectId"`
	ProjectName     string `json:"projectName"`
	EnvironmentID   string `json:"environmentId"`
	EnvironmentName string `json:"environmentName"`
}

type ProjectConfig struct {
	OrgID           string         `json:"orgId"`
	EnvironmentSlug string         `json:"environmentSlug"`
	Projects        []ProjectGrant `json:"projects"`
}

// LoadProjectDir reads .itapu.json from dir itself (no parent walk, unlike
// FindProject). Returns (nil, nil) when the file does not exist.
func LoadProjectDir(dir string) (*ProjectConfig, error) {
	path := filepath.Join(dir, ProjectConfigName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt %s: %w", path, err)
	}
	return &cfg, nil
}

// FindProject walks up from dir looking for .itapu.json, like git does.
func FindProject(dir string) (*ProjectConfig, string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", err
	}
	for {
		path := filepath.Join(dir, ProjectConfigName)
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg ProjectConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, "", fmt.Errorf("corrupt %s: %w", path, err)
			}
			return &cfg, path, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, "", fmt.Errorf("no %s found in this directory or any parent — run `itapu init` first", ProjectConfigName)
		}
		dir = parent
	}
}

func SaveProject(dir string, cfg *ProjectConfig) (string, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, ProjectConfigName)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
