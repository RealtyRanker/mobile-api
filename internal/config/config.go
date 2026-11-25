package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type LoggingConfig struct {
	Level    string `yaml:"level"`
	FilePath string `yaml:"file_path"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type MetricsConfig struct {
	Port int `yaml:"port"`
}

// AuthConfig holds the secret used to sign the app's own access tokens.
// There's no external identity provider to configure: sign-in is a random
// per-device id the app generates itself (see deviceSignInRequest) rather
// than Sign in with Apple, which requires a paid Apple Developer Program
// membership that isn't worth it for a single-user personal app.
type AuthConfig struct {
	JWTSecretFile       string `yaml:"jwt_secret_file"`
	JWTSecret           string `yaml:"-"`
	AccessTokenTTLMins  int    `yaml:"access_token_ttl_minutes"`
	RefreshTokenTTLDays int    `yaml:"refresh_token_ttl_days"`
}

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
	Server   ServerConfig   `yaml:"server"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Auth     AuthConfig     `yaml:"auth"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Auth.JWTSecretFile == "" {
		return nil, fmt.Errorf("auth.jwt_secret_file is not set in config")
	}
	secretBytes, err := os.ReadFile(cfg.Auth.JWTSecretFile)
	if err != nil {
		return nil, fmt.Errorf("reading jwt secret file %s: %w", cfg.Auth.JWTSecretFile, err)
	}
	cfg.Auth.JWTSecret = strings.TrimSpace(string(secretBytes))
	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("jwt secret file %s is empty", cfg.Auth.JWTSecretFile)
	}
	if cfg.Auth.AccessTokenTTLMins == 0 {
		cfg.Auth.AccessTokenTTLMins = 60
	}
	if cfg.Auth.RefreshTokenTTLDays == 0 {
		cfg.Auth.RefreshTokenTTLDays = 90
	}
	return &cfg, nil
}
