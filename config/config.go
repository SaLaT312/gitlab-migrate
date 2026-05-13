package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type TransportType string

const (
	TransportSSH   TransportType = "ssh"
	TransportHTTPS TransportType = "https"
)

type LogFormat string

const (
	LogFormatPlaintext LogFormat = "plaintext"
	LogFormatJSON      LogFormat = "json"
)

type LogOutput string

const (
	LogOutputStdout LogOutput = "stdout"
	LogOutputFile   LogOutput = "file"
	LogOutputBoth   LogOutput = "both"
)

type GitLabConfig struct {
	URL       string        `yaml:"url"`
	Token     string        `yaml:"token"`
	Transport TransportType `yaml:"transport"`
	SSHPort   int           `yaml:"ssh_port,omitempty"`
	SSHKey    string        `yaml:"ssh_key,omitempty"`
}

type LogConfig struct {
	Format   LogFormat `yaml:"format"`
	Output   LogOutput `yaml:"output"`
	File     string    `yaml:"file,omitempty"`
	JSONUnix bool      `yaml:"json_unix_timestamp,omitempty"`
}

type GroupFilter struct {
	Path             string   `yaml:"path"`
	IncludeSubgroups []string `yaml:"include_subgroups,omitempty"`
	IncludeProjects  []string `yaml:"include_projects,omitempty"`
	ExcludeSubgroups []string `yaml:"exclude_subgroups,omitempty"`
	ExcludeProjects  []string `yaml:"exclude_projects,omitempty"`
}

type Config struct {
	Source      GitLabConfig  `yaml:"source"`
	Target      GitLabConfig  `yaml:"target"`
	ParentGroup string        `yaml:"parent_group"`
	Groups      []GroupFilter `yaml:"groups"`
	Log         LogConfig     `yaml:"log"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Source.URL == "" {
		return fmt.Errorf("source.url is required")
	}
	if c.Source.Token == "" {
		return fmt.Errorf("source.token is required")
	}
	if c.Target.URL == "" {
		return fmt.Errorf("target.url is required")
	}
	if c.Target.Token == "" {
		return fmt.Errorf("target.token is required")
	}
	if c.ParentGroup == "" {
		return fmt.Errorf("parent_group is required")
	}

	if c.Source.Transport == "" {
		c.Source.Transport = TransportSSH
	}
	if c.Target.Transport == "" {
		c.Target.Transport = TransportSSH
	}
	if c.Source.Transport != TransportSSH && c.Source.Transport != TransportHTTPS {
		return fmt.Errorf("source.transport must be 'ssh' or 'https'")
	}
	if c.Target.Transport != TransportSSH && c.Target.Transport != TransportHTTPS {
		return fmt.Errorf("target.transport must be 'ssh' or 'https'")
	}
	if c.Source.SSHPort == 0 {
		c.Source.SSHPort = 22
	}
	if c.Target.SSHPort == 0 {
		c.Target.SSHPort = 22
	}
	if c.Source.SSHKey == "" {
		c.Source.SSHKey = os.Getenv("HOME") + "/.ssh/id_rsa"
	}
	if c.Target.SSHKey == "" {
		c.Target.SSHKey = os.Getenv("HOME") + "/.ssh/id_rsa"
	}
	if c.Log.Format == "" {
		c.Log.Format = LogFormatPlaintext
	}
	if c.Log.Output == "" {
		c.Log.Output = LogOutputStdout
	}

	return nil
}
