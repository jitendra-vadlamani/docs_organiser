package config

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type ModelDefinition struct {
	Name   string `json:"name"`
	URL    string `json:"url"` // OpenAI-compatible endpoint
	Status string `json:"status"`
}

type Config struct {
	// Infra Settings (Loaded from YAML/Env)
	APIURL         string `mapstructure:"api" json:"api"`
	MetricsEnabled bool   `mapstructure:"metrics_enabled" json:"metrics_enabled"`
	MetricsPort    int    `mapstructure:"metrics_port" json:"metrics_port"`
	ServerPort     int    `mapstructure:"server_port" json:"server_port"`
	Encoding       string `mapstructure:"encoding" json:"encoding"`
	ContextWindow  int    `mapstructure:"ctx" json:"ctx"`
	DBPath         string `mapstructure:"db_path" json:"db_path"`

	// User Settings (Managed via UI/API, initialized to defaults)
	SourceDir        string            `mapstructure:"-" json:"src"`
	DestDir          string            `mapstructure:"-" json:"dst"`
	AllowedModels    []ModelDefinition `mapstructure:"-" json:"allowed_models"`
	DefaultModelName string            `mapstructure:"-" json:"default_model_name"`
	Workers          int               `mapstructure:"-" json:"workers"`
	ExtractLimit     int               `mapstructure:"-" json:"limit"`
	Categories       []string          `mapstructure:"-" json:"categories"`
}

func LoadConfig() (*Config, error) {
	// 1. Set Infrastructure Defaults
	viper.SetDefault("api", "http://localhost:8080/v1")
	viper.SetDefault("metrics_enabled", true)
	viper.SetDefault("metrics_port", 8081)
	viper.SetDefault("server_port", 8090)
	viper.SetDefault("encoding", "cl100k_base")
	viper.SetDefault("ctx", 4096)
	viper.SetDefault("db_path", "data/badger")

	// User Defaults (These will not be loaded from YAML)
	var defaultModels []ModelDefinition
	if runtime.GOOS == "darwin" {
		defaultModels = []ModelDefinition{
			{Name: "mlx-community/Llama-3.2-1B-Instruct-4bit", URL: "http://localhost:8080/v1"},
		}
	} else {
		defaultModels = []ModelDefinition{}
	}
	defaultWorkers := 5
	defaultLimit := 0 // 0 means "Auto"

	// 2. Define Flags for Infra
	pflag.String("api", "http://localhost:8080/v1", "URL of the MLX server")
	pflag.Bool("metrics_enabled", true, "Enable Prometheus metrics")
	pflag.Int("metrics_port", 8081, "Port for Prometheus metrics")
	pflag.Int("server_port", 8090, "Port for the app server")
	pflag.String("db_path", "data/badger", "Path to Badger KV database")
	configPath := pflag.String("config", "config.yaml", "Path to YAML configuration file")
	pflag.Parse()

	// 3. Bind Flags to Viper
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return nil, fmt.Errorf("failed to bind flags: %w", err)
	}

	// 4. Environment variables (Prefix DOCS_)
	viper.SetEnvPrefix("DOCS")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	// 5. Load Configuration File (Infra Only)
	if *configPath != "" {
		viper.SetConfigFile(*configPath)
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				// We don't fail if config is missing, just use env/defaults
				fmt.Printf("[*] Info: Config file not found or unreadable: %v. Using defaults.\n", err)
			}
		}
	}

	// 6. Unmarshal into struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 7. Manually initialize user settings to defaults
	// These are intentionally not bound to Viper/YAML to ensure they come from UI
	cfg.AllowedModels = defaultModels
	if len(defaultModels) > 0 {
		cfg.DefaultModelName = defaultModels[0].Name
	}
	cfg.Workers = defaultWorkers
	cfg.ExtractLimit = defaultLimit
	cfg.Categories = []string{} // Initialized empty for auto-discovery

	return &cfg, nil
}
