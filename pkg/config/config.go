package config

import (
	"gopkg.in/yaml.v2"
)

type Config struct {
	LogLevel       string `yaml:"log_level"`
	Port           int    `yaml:"port"`
	ListenAddr     string `yaml:"listen_address"`
	RemoteURL      string `yaml:"remote_write_url"`
	RemoteUser     string `yaml:"remote_write_user"`
	RemotePassword string `yaml:"remote_write_password"`
}

func ConvertConfig(cfg []byte) (*Config, error) {
	c := &Config{}
	err := yaml.Unmarshal(cfg, c)
	return c, err
}
