package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/source/aliyun"
	awssource "github.com/ylighgh/cloud-sd/internal/source/aws"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Collector CollectorConfig `yaml:"collector"`
	Routing   RoutingConfig   `yaml:"routing"`
	Aliyun    AliyunConfig    `yaml:"aliyun"`
	AWS       AWSConfig       `yaml:"aws"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type CollectorConfig struct {
	Scopes          []string      `yaml:"scopes"`
	Engines         EngineConfig  `yaml:"engines"`
	EnginesSet      bool          `yaml:"-"`
	RefreshInterval time.Duration `yaml:"-"`
	RefreshTimeout  time.Duration `yaml:"-"`
	RequestTimeout  time.Duration `yaml:"-"`
}

type EngineConfig struct {
	Redis    bool `yaml:"redis"`
	Postgres bool `yaml:"postgres"`
	MySQL    bool `yaml:"mysql"`
	Mongo    bool `yaml:"mongo"`
	Node     bool `yaml:"node"`
}

func (e EngineConfig) Set() core.EngineSet {
	return core.EngineSet{
		core.EngineRedis:    e.Redis,
		core.EnginePostgres: e.Postgres,
		core.EngineMySQL:    e.MySQL,
		core.EngineMongo:    e.Mongo,
		core.EngineNode:     e.Node,
	}
}

func (e EngineConfig) AnyEnabled() bool {
	return e.Redis || e.Postgres || e.MySQL || e.Mongo || e.Node
}

func (c *CollectorConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawCollectorConfig struct {
		Scopes          []string      `yaml:"scopes"`
		Engines         *EngineConfig `yaml:"engines"`
		RefreshInterval string        `yaml:"refresh_interval"`
		RefreshTimeout  string        `yaml:"refresh_timeout"`
		RequestTimeout  string        `yaml:"request_timeout"`
	}
	var raw rawCollectorConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	c.Scopes = raw.Scopes
	c.Engines = EngineConfig{Redis: true}
	c.EnginesSet = raw.Engines != nil
	if raw.Engines != nil {
		c.Engines = *raw.Engines
	}
	c.RefreshInterval = 5 * time.Minute
	c.RefreshTimeout = time.Minute
	c.RequestTimeout = 20 * time.Second
	if raw.RefreshInterval != "" {
		d, err := time.ParseDuration(raw.RefreshInterval)
		if err != nil {
			return fmt.Errorf("collector.refresh_interval: %w", err)
		}
		c.RefreshInterval = d
	}
	if raw.RefreshTimeout != "" {
		d, err := time.ParseDuration(raw.RefreshTimeout)
		if err != nil {
			return fmt.Errorf("collector.refresh_timeout: %w", err)
		}
		c.RefreshTimeout = d
	}
	if raw.RequestTimeout != "" {
		d, err := time.ParseDuration(raw.RequestTimeout)
		if err != nil {
			return fmt.Errorf("collector.request_timeout: %w", err)
		}
		c.RequestTimeout = d
	}
	return nil
}

type RoutingConfig struct {
	ScopeTag   string `yaml:"scope_tag"`
	DisableTag string `yaml:"disable_tag"`
}

type AliyunConfig struct {
	Enabled  bool                   `yaml:"enabled"`
	Accounts []aliyun.AccountConfig `yaml:"accounts"`
}

type AWSConfig struct {
	Enabled  bool                      `yaml:"enabled"`
	Accounts []awssource.AccountConfig `yaml:"accounts"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.Collector.RefreshInterval == 0 {
		c.Collector.RefreshInterval = 5 * time.Minute
	}
	if c.Collector.RefreshTimeout == 0 {
		c.Collector.RefreshTimeout = time.Minute
	}
	if c.Collector.RequestTimeout == 0 {
		c.Collector.RequestTimeout = 20 * time.Second
	}
	if !c.Collector.EnginesSet && c.Collector.Engines == (EngineConfig{}) {
		c.Collector.Engines.Redis = true
	}
	if c.Routing.ScopeTag == "" {
		c.Routing.ScopeTag = "cloud_sd_scope"
	}
	if c.Routing.DisableTag == "" {
		c.Routing.DisableTag = "cloud_sd_disable"
	}
}

func (c Config) Validate() error {
	if c.Collector.RefreshInterval <= 0 {
		return fmt.Errorf("collector.refresh_interval must be positive")
	}
	if c.Collector.RefreshTimeout <= 0 {
		return fmt.Errorf("collector.refresh_timeout must be positive")
	}
	if c.Collector.RequestTimeout <= 0 {
		return fmt.Errorf("collector.request_timeout must be positive")
	}
	if c.Routing.ScopeTag == "" {
		return fmt.Errorf("routing.scope_tag must not be empty")
	}
	if c.Routing.DisableTag == "" {
		return fmt.Errorf("routing.disable_tag must not be empty")
	}
	if !c.Collector.Engines.AnyEnabled() {
		return fmt.Errorf("at least one collector engine must be enabled")
	}
	if !c.Aliyun.Enabled && !c.AWS.Enabled {
		return fmt.Errorf("at least one provider must be enabled")
	}
	if c.Aliyun.Enabled {
		if err := validateAliyunAccounts(c.Aliyun.Accounts); err != nil {
			return err
		}
	}
	if c.AWS.Enabled {
		if err := validateAWSAccounts(c.AWS.Accounts); err != nil {
			return err
		}
	}
	return nil
}

func validateAliyunAccounts(accounts []aliyun.AccountConfig) error {
	if len(accounts) == 0 {
		return fmt.Errorf("aliyun.accounts must not be empty")
	}
	seen := map[string]struct{}{}
	for i, account := range accounts {
		if account.Name == "" {
			return fmt.Errorf("aliyun.accounts[%d].name must not be empty", i)
		}
		if _, ok := seen[account.Name]; ok {
			return fmt.Errorf("aliyun account %q is duplicated", account.Name)
		}
		seen[account.Name] = struct{}{}
		if len(account.Regions) == 0 {
			return fmt.Errorf("aliyun account %q must configure at least one region", account.Name)
		}
		if _, err := account.ResolveCredentials(); err != nil {
			return err
		}
	}
	return nil
}

func validateAWSAccounts(accounts []awssource.AccountConfig) error {
	if len(accounts) == 0 {
		return fmt.Errorf("aws.accounts must not be empty")
	}
	seen := map[string]struct{}{}
	for i, account := range accounts {
		if account.Name == "" {
			return fmt.Errorf("aws.accounts[%d].name must not be empty", i)
		}
		if _, ok := seen[account.Name]; ok {
			return fmt.Errorf("aws account %q is duplicated", account.Name)
		}
		seen[account.Name] = struct{}{}
		if len(account.Regions) == 0 {
			return fmt.Errorf("aws account %q must configure at least one region", account.Name)
		}
		if _, err := account.ResolveCredentials(); err != nil {
			return err
		}
	}
	return nil
}
