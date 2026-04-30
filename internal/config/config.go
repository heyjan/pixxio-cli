package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

type ctxKey struct{}

// Config holds pixx CLI configuration.
type Config struct {
	Mediaspace string `json:"mediaspace,omitempty"`
	APIKey     string `json:"apiKey,omitempty"`
}

// Load reads config from XDG_CONFIG_HOME/pixx/config.json, with env overrides.
func Load() (*Config, error) {
	cfg := &Config{}
	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnv(cfg), nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return applyEnv(cfg), nil
}

// Save writes config atomically to the config file.
func Save(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}

func applyEnv(cfg *Config) *Config {
	if v := os.Getenv("PIXX_MEDIASPACE"); v != "" {
		cfg.Mediaspace = v
	}
	if v := os.Getenv("PIXX_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	return cfg
}

func configDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "pixx")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// WithConfig stores config in context.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, ctxKey{}, cfg)
}

// FromContext retrieves config from context.
func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(ctxKey{}).(*Config)
	return cfg
}
