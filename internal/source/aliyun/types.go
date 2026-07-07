package aliyun

import (
	"fmt"
	"os"
)

type AccountConfig struct {
	Name               string   `yaml:"name"`
	Regions            []string `yaml:"regions"`
	AccessKeyID        string   `yaml:"access_key_id"`
	AccessKeySecret    string   `yaml:"access_key_secret"`
	AccessKeyIDEnv     string   `yaml:"access_key_id_env"`
	AccessKeySecretEnv string   `yaml:"access_key_secret_env"`
}

type Credentials struct {
	AccessKeyID     string
	AccessKeySecret string
}

func (a AccountConfig) ResolveCredentials() (Credentials, error) {
	hasDirect := a.AccessKeyID != "" || a.AccessKeySecret != ""
	hasEnv := a.AccessKeyIDEnv != "" || a.AccessKeySecretEnv != ""
	if hasDirect && hasEnv {
		return Credentials{}, fmt.Errorf("aliyun account %q mixes direct and environment credentials", a.Name)
	}
	if hasDirect {
		if a.AccessKeyID == "" || a.AccessKeySecret == "" {
			return Credentials{}, fmt.Errorf("aliyun account %q direct credentials are incomplete", a.Name)
		}
		return Credentials{AccessKeyID: a.AccessKeyID, AccessKeySecret: a.AccessKeySecret}, nil
	}
	if hasEnv {
		if a.AccessKeyIDEnv == "" || a.AccessKeySecretEnv == "" {
			return Credentials{}, fmt.Errorf("aliyun account %q environment credential names are incomplete", a.Name)
		}
		accessKeyID := os.Getenv(a.AccessKeyIDEnv)
		accessKeySecret := os.Getenv(a.AccessKeySecretEnv)
		if accessKeyID == "" || accessKeySecret == "" {
			return Credentials{}, fmt.Errorf("aliyun account %q environment credentials are empty", a.Name)
		}
		return Credentials{AccessKeyID: accessKeyID, AccessKeySecret: accessKeySecret}, nil
	}
	return Credentials{}, fmt.Errorf("aliyun account %q has no credentials", a.Name)
}
