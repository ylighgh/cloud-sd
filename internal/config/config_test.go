package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadParsesYAMLConfigWithDirectAndEnvCredentials(t *testing.T) {
	t.Setenv("ALIYUN_GAME_ACCESS_KEY_ID", "env-ak")
	t.Setenv("ALIYUN_GAME_ACCESS_KEY_SECRET", "env-sk")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  listen: ":8080"
collector:
  scopes:
    - id1
    - game-id1
  engines:
    redis: true
    mysql: true
    postgres: true
    mongo: true
    node: true
  refresh_interval: 5m
  refresh_timeout: 2m
  request_timeout: 20s
routing:
  scope_tag: cloud_sd_scope
  disable_tag: cloud_sd_disable
aliyun:
  enabled: true
  accounts:
    - name: prod
      regions:
        - cn-hangzhou
        - ap-southeast-1
      access_key_id: direct-ak
      access_key_secret: direct-sk
    - name: game
      regions:
        - cn-shanghai
      access_key_id_env: ALIYUN_GAME_ACCESS_KEY_ID
      access_key_secret_env: ALIYUN_GAME_ACCESS_KEY_SECRET
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Listen != ":8080" {
		t.Fatalf("listen = %q", cfg.Server.Listen)
	}
	if cfg.Collector.RefreshInterval != 5*time.Minute {
		t.Fatalf("refresh interval = %s", cfg.Collector.RefreshInterval)
	}
	if cfg.Collector.RefreshTimeout != 2*time.Minute {
		t.Fatalf("refresh timeout = %s", cfg.Collector.RefreshTimeout)
	}
	if cfg.Collector.RequestTimeout != 20*time.Second {
		t.Fatalf("request timeout = %s", cfg.Collector.RequestTimeout)
	}
	if !cfg.Collector.Engines.Redis || !cfg.Collector.Engines.MySQL || !cfg.Collector.Engines.Postgres || !cfg.Collector.Engines.Mongo || !cfg.Collector.Engines.Node {
		t.Fatalf("engines = %#v, want all enabled", cfg.Collector.Engines)
	}
	if len(cfg.Aliyun.Accounts) != 2 {
		t.Fatalf("accounts len = %d", len(cfg.Aliyun.Accounts))
	}

	prod, err := cfg.Aliyun.Accounts[0].ResolveCredentials()
	if err != nil {
		t.Fatalf("resolve direct credentials: %v", err)
	}
	if prod.AccessKeyID != "direct-ak" || prod.AccessKeySecret != "direct-sk" {
		t.Fatalf("direct credentials = %#v", prod)
	}

	game, err := cfg.Aliyun.Accounts[1].ResolveCredentials()
	if err != nil {
		t.Fatalf("resolve env credentials: %v", err)
	}
	if game.AccessKeyID != "env-ak" || game.AccessKeySecret != "env-sk" {
		t.Fatalf("env credentials = %#v", game)
	}
}

func TestLoadDefaultsCollectorEnginesToRedisOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  listen: ":8080"
collector:
  refresh_interval: 5m
  request_timeout: 20s
aliyun:
  enabled: true
  accounts:
    - name: prod
      regions: [cn-hangzhou]
      access_key_id: direct-ak
      access_key_secret: direct-sk
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Collector.Engines.Redis {
		t.Fatalf("redis engine = false, want true")
	}
	if cfg.Collector.Engines.MySQL || cfg.Collector.Engines.Postgres || cfg.Collector.Engines.Mongo || cfg.Collector.Engines.Node {
		t.Fatalf("engines = %#v, want only redis enabled", cfg.Collector.Engines)
	}
}

func TestLoadParsesAWSConfigWithoutAliyun(t *testing.T) {
	t.Setenv("AWS_GAME_ACCESS_KEY_ID", "env-ak")
	t.Setenv("AWS_GAME_SECRET_ACCESS_KEY", "env-sk")
	t.Setenv("AWS_GAME_SESSION_TOKEN", "env-token")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  listen: ":8080"
collector:
  engines:
    redis: true
    mysql: true
    postgres: true
    mongo: true
    node: true
aws:
  enabled: true
  accounts:
    - name: prod
      regions:
        - ap-southeast-1
      access_key_id: direct-ak
      secret_access_key: direct-sk
    - name: game
      regions:
        - us-east-1
      access_key_id_env: AWS_GAME_ACCESS_KEY_ID
      secret_access_key_env: AWS_GAME_SECRET_ACCESS_KEY
      session_token_env: AWS_GAME_SESSION_TOKEN
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Aliyun.Enabled {
		t.Fatalf("aliyun enabled = true, want false")
	}
	if !cfg.AWS.Enabled {
		t.Fatalf("aws enabled = false, want true")
	}
	if len(cfg.AWS.Accounts) != 2 {
		t.Fatalf("aws accounts len = %d, want 2", len(cfg.AWS.Accounts))
	}

	prod, err := cfg.AWS.Accounts[0].ResolveCredentials()
	if err != nil {
		t.Fatalf("resolve direct aws credentials: %v", err)
	}
	if prod.AccessKeyID != "direct-ak" || prod.SecretAccessKey != "direct-sk" || prod.SessionToken != "" {
		t.Fatalf("direct aws credentials = %#v", prod)
	}

	game, err := cfg.AWS.Accounts[1].ResolveCredentials()
	if err != nil {
		t.Fatalf("resolve env aws credentials: %v", err)
	}
	if game.AccessKeyID != "env-ak" || game.SecretAccessKey != "env-sk" || game.SessionToken != "env-token" {
		t.Fatalf("env aws credentials = %#v", game)
	}
}

func TestLoadParsesAWSAccessKeySecretAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  listen: ":8080"
collector:
  engines:
    redis: true
aws:
  enabled: true
  accounts:
    - name: aws-test
      regions:
        - ap-southeast-1
      access_key_id: direct-ak
      access_key_secret: direct-sk
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	credentials, err := cfg.AWS.Accounts[0].ResolveCredentials()
	if err != nil {
		t.Fatalf("resolve direct aws credentials: %v", err)
	}
	if credentials.AccessKeyID != "direct-ak" || credentials.SecretAccessKey != "direct-sk" {
		t.Fatalf("direct aws credentials = %#v", credentials)
	}
}

func TestLoadAllowsEmptyOrOmittedCollectorScopes(t *testing.T) {
	tests := map[string]string{
		"empty": `
server:
  listen: ":8080"
collector:
  scopes: []
routing:
  scope_tag: cloud_sd_scope
  disable_tag: cloud_sd_disable
aliyun:
  enabled: true
  accounts:
    - name: prod
      regions: [cn-hangzhou]
      access_key_id: direct-ak
      access_key_secret: direct-sk
`,
		"omitted": `
server:
  listen: ":8080"
collector:
  refresh_interval: 5m
  request_timeout: 20s
routing:
  scope_tag: cloud_sd_scope
  disable_tag: cloud_sd_disable
aliyun:
  enabled: true
  accounts:
    - name: prod
      regions: [cn-hangzhou]
      access_key_id: direct-ak
      access_key_secret: direct-sk
`,
	}

	for name, yamlData := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(yamlData), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(cfg.Collector.Scopes) != 0 {
				t.Fatalf("scopes len = %d, want 0", len(cfg.Collector.Scopes))
			}
		})
	}
}

func TestLoadRejectsConfigWithNoEnabledCollectorEngines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  listen: ":8080"
collector:
  engines:
    redis: false
    mysql: false
    postgres: false
    mongo: false
    node: false
aliyun:
  enabled: true
  accounts:
    - name: prod
      regions: [cn-hangzhou]
      access_key_id: direct-ak
      access_key_secret: direct-sk
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want no enabled collector engines error")
	}
}

func TestLoadRejectsMixedCredentialModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
server:
  listen: ":8080"
collector:
  scopes: [id1]
routing:
  scope_tag: cloud_sd_scope
  disable_tag: cloud_sd_disable
aliyun:
  enabled: true
  accounts:
    - name: prod
      regions: [cn-hangzhou]
      access_key_id: direct-ak
      access_key_secret: direct-sk
      access_key_id_env: ALIYUN_ACCESS_KEY_ID
      access_key_secret_env: ALIYUN_ACCESS_KEY_SECRET
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want mixed credential mode error")
	}
}
