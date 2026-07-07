package aws

import (
	"context"
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/identity"
)

type ClientOptions struct {
	RequestTimeout time.Duration
}

type identityClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
}

func resolveAccountID(ctx context.Context, cache identity.Cache, account AccountConfig, client identityClient) (string, error) {
	key, err := identityKey(account)
	if err != nil {
		return "", err
	}
	cached, ok := cache.Get(key)
	if ok {
		return cached.AccountID, nil
	}
	accountID, err := client.GetCallerIdentity(ctx)
	if err != nil {
		return "", err
	}
	cache.Set(key, identity.Identity{
		Provider:   core.ProviderAWS,
		AccountID:  accountID,
		ResolvedAt: time.Now(),
	})
	return accountID, nil
}

func identityKey(account AccountConfig) (identity.Key, error) {
	credentials, err := account.ResolveCredentials()
	if err != nil {
		return identity.Key{}, err
	}
	return identity.Key{
		Provider:    core.ProviderAWS,
		AccountName: account.Name,
		AccessKeyID: credentials.AccessKeyID,
	}, nil
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
