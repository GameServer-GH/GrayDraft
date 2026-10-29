package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config for Console BFF.
type Config struct {
	App struct {
		Name string `yaml:"name"`
		Env  string `yaml:"env"`
	} `yaml:"app"`
	HTTP struct {
		Port int `yaml:"port"`
	} `yaml:"http"`
	Core struct {
		URL string `yaml:"url"`
	} `yaml:"core"`
	Static struct {
		Dir string `yaml:"dir"`
	} `yaml:"static"`
}

// Load reads config from path and env.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 13000
	}
	if cfg.Core.URL == "" {
		cfg.Core.URL = "http://localhost:18080"
	}
	if cfg.Static.Dir == "" {
		cfg.Static.Dir = "static"
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.HTTP.Port)
	}
	if v := os.Getenv("CORE_URL"); v != "" {
		cfg.Core.URL = v
	}
	if v := os.Getenv("STATIC_DIR"); v != "" {
		cfg.Static.Dir = v
	}
	return cfg, nil
}
