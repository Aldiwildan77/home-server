package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
)

const DefaultConfigFileName string = "config.yaml"

var DefaultConfigPath = func() string {
	rootDir, _ := os.Getwd()
	return filepath.Join(rootDir, DefaultConfigFileName)
}

type Config struct {
	Output    OutputConfig    `yaml:"output"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	Interval  time.Duration   `yaml:"interval"`
}

type DiscoveryConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type OutputConfig struct {
	Path string `yaml:"path"`
}

func Load(path string) (cfg *Config, err error) {
	// Setup default
	cfg = &Config{
		Interval: 3 * time.Second,
		Output: OutputConfig{
			Path: "./out/hosts.json",
		},
		Discovery: DiscoveryConfig{
			Command: "ip",
			Args:    []string{"route"},
		},
	}

	configFile, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}

		return
	}

	err = yaml.Unmarshal(configFile, cfg)
	return
}
