package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
}

type GRPCConfig struct {
	Port            string `mapstructure:"port"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	URL            string `mapstructure:"url"`
	MigrationsPath string `mapstructure:"migrations_path"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads the config file and overrides values with environment variables if set.
func Load(path string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("grpc.port", "50051")
	v.SetDefault("grpc.shutdown_timeout", 10)
	v.SetDefault("database.migrations_path", "migrations")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Config file
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Environment variables override file values.
	// GRPC_PORT -> grpc.port, DATABASE_URL -> database.url, etc.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("database url is required (set in config file or DATABASE_URL env)")
	}

	return cfg, nil
}
