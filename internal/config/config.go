package config

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	SourceDir     string   `mapstructure:"src"`
	DestDir       string   `mapstructure:"dst"`
	APIURL        string   `mapstructure:"api"`
	ModelName     string   `mapstructure:"model"`
	ContextWindow int      `mapstructure:"ctx"`
	Encoding      string   `mapstructure:"encoding"`
	Workers       int      `mapstructure:"workers"`
	ExtractLimit  int      `mapstructure:"limit"`
	Categories    []string `mapstructure:"categories"`
}

func LoadConfig() (*Config, error) {
	// 1. Set Defaults
	viper.SetDefault("api", "http://localhost:8080/v1")
	viper.SetDefault("model", "mlx-community/Llama-3.2-1B-Instruct-4bit")
	viper.SetDefault("ctx", 4096)
	viper.SetDefault("encoding", "cl100k_base")
	viper.SetDefault("workers", 5)
	viper.SetDefault("limit", 100000)

	// 2. Define Flags using pflag
	pflag.String("src", "", "Source directory to scan for files")
	pflag.String("dst", "", "Destination directory to move files into")
	pflag.String("api", "http://localhost:8080/v1", "URL of the MLX server")
	pflag.String("model", "mlx-community/Llama-3.2-1B-Instruct-4bit", "Model name to use in API requests")
	pflag.Int("ctx", 4096, "Model context window (max tokens)")
	pflag.String("encoding", "cl100k_base", "Tiktoken encoding to use (e.g. cl100k_base, p50k_base)")
	pflag.Int("workers", 5, "Number of concurrent workers")
	pflag.Int("limit", 100000, "Max characters to extract from each file")
	pflag.StringSlice("categories", []string{}, "Manual list of allowed categories (comma-separated)")
	configPath := pflag.String("config", "", "Path to YAML configuration file")
	pflag.Parse()

	// 3. Bind Flags to Viper
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return nil, fmt.Errorf("failed to bind flags: %w", err)
	}

	// 4. Environment variables (Prefix DOCS_)
	viper.SetEnvPrefix("DOCS")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// 5. Load Configuration File
	if *configPath != "" {
		// If explicit config path provided, use it
		viper.SetConfigFile(*configPath)
	} else {
		// Otherwise look for config.yaml in current directory or project root
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		// It's okay if the config file is missing unless it was explicitly requested
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && *configPath != "" {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 6. Unmarshal into struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
