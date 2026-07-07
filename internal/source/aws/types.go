package aws

import (
	"fmt"
	"os"
)

type AccountConfig struct {
	Name               string   `yaml:"name"`
	Regions            []string `yaml:"regions"`
	AccessKeyID        string   `yaml:"access_key_id"`
	SecretAccessKey    string   `yaml:"secret_access_key"`
	AccessKeySecret    string   `yaml:"access_key_secret"`
	SessionToken       string   `yaml:"session_token"`
	AccessKeyIDEnv     string   `yaml:"access_key_id_env"`
	SecretAccessKeyEnv string   `yaml:"secret_access_key_env"`
	AccessKeySecretEnv string   `yaml:"access_key_secret_env"`
	SessionTokenEnv    string   `yaml:"session_token_env"`
}

type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func (a AccountConfig) ResolveCredentials() (Credentials, error) {
	secretAccessKey, err := resolveAWSCredentialAlias(a.Name, "secret_access_key", a.SecretAccessKey, "access_key_secret", a.AccessKeySecret)
	if err != nil {
		return Credentials{}, err
	}
	secretAccessKeyEnv, err := resolveAWSCredentialAlias(a.Name, "secret_access_key_env", a.SecretAccessKeyEnv, "access_key_secret_env", a.AccessKeySecretEnv)
	if err != nil {
		return Credentials{}, err
	}

	hasDirect := a.AccessKeyID != "" || secretAccessKey != "" || a.SessionToken != ""
	hasEnv := a.AccessKeyIDEnv != "" || secretAccessKeyEnv != "" || a.SessionTokenEnv != ""
	if hasDirect && hasEnv {
		return Credentials{}, fmt.Errorf("aws account %q mixes direct and environment credentials", a.Name)
	}
	if hasDirect {
		if a.AccessKeyID == "" || secretAccessKey == "" {
			return Credentials{}, fmt.Errorf("aws account %q direct credentials are incomplete", a.Name)
		}
		return Credentials{
			AccessKeyID:     a.AccessKeyID,
			SecretAccessKey: secretAccessKey,
			SessionToken:    a.SessionToken,
		}, nil
	}
	if hasEnv {
		if a.AccessKeyIDEnv == "" || secretAccessKeyEnv == "" {
			return Credentials{}, fmt.Errorf("aws account %q environment credential names are incomplete", a.Name)
		}
		accessKeyID := os.Getenv(a.AccessKeyIDEnv)
		secretAccessKey := os.Getenv(secretAccessKeyEnv)
		sessionToken := ""
		if a.SessionTokenEnv != "" {
			sessionToken = os.Getenv(a.SessionTokenEnv)
		}
		if accessKeyID == "" || secretAccessKey == "" {
			return Credentials{}, fmt.Errorf("aws account %q environment credentials are empty", a.Name)
		}
		return Credentials{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
			SessionToken:    sessionToken,
		}, nil
	}
	return Credentials{}, fmt.Errorf("aws account %q has no credentials", a.Name)
}

func resolveAWSCredentialAlias(accountName, primaryName, primaryValue, aliasName, aliasValue string) (string, error) {
	if primaryValue == "" {
		return aliasValue, nil
	}
	if aliasValue == "" || aliasValue == primaryValue {
		return primaryValue, nil
	}
	return "", fmt.Errorf("aws account %q configures both %s and %s with different values", accountName, primaryName, aliasName)
}
