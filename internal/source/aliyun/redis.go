package aliyun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
)

const redisResourceType = "redis_instance"

type CloudClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeRedisInstances(ctx context.Context) ([]RedisInstance, error)
}

type ClientOptions struct {
	RequestTimeout time.Duration
}

type ClientFactory func(account AccountConfig, region string, opts ClientOptions) (CloudClient, error)

type RedisInstance struct {
	InstanceID       string
	InstanceName     string
	RegionID         string
	ConnectionDomain string
	Port             int
	InstanceType     string
	Tags             map[string]string
}

type RedisSource struct {
	accounts       []AccountConfig
	clientFactory  ClientFactory
	requestTimeout time.Duration
	identityCache  identity.Cache
}

type Option func(*RedisSource)

func WithClientFactory(factory ClientFactory) Option {
	return func(source *RedisSource) {
		source.clientFactory = factory
	}
}

func WithRequestTimeout(timeout time.Duration) Option {
	return func(source *RedisSource) {
		source.requestTimeout = timeout
	}
}

func WithIdentityCache(cache identity.Cache) Option {
	return func(source *RedisSource) {
		source.identityCache = cache
	}
}

func NewRedisSource(accounts []AccountConfig, opts ...Option) *RedisSource {
	source := &RedisSource{
		accounts:       append([]AccountConfig(nil), accounts...),
		clientFactory:  NewSDKClient,
		requestTimeout: 20 * time.Second,
		identityCache:  identity.NewMemoryCache(),
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

func (s *RedisSource) Name() string {
	return "aliyun-redis"
}

func (s *RedisSource) Provider() core.Provider {
	return core.ProviderAliyun
}

func (s *RedisSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		if len(account.Regions) == 0 {
			continue
		}

		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aliyun client for account %q region %q: %w", account.Name, region, err))
				continue
			}
			identityKey, err := s.identityKey(account)
			if err != nil {
				errs = append(errs, fmt.Errorf("build identity cache key for account %q region %q: %w", account.Name, region, err))
				continue
			}
			cached, ok := s.identityCache.Get(identityKey)
			accountID := cached.AccountID
			if !ok {
				accountID, err = client.GetCallerIdentity(ctx)
				if err != nil {
					errs = append(errs, fmt.Errorf("resolve aliyun account identity for account %q region %q: %w", account.Name, region, err))
					continue
				}
				s.identityCache.Set(identityKey, identity.Identity{
					Provider:   core.ProviderAliyun,
					AccountID:  accountID,
					ResolvedAt: time.Now(),
				})
			}

			instances, err := client.DescribeRedisInstances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aliyun redis instances for account %q region %q: %w", account.Name, region, err))
				continue
			}
			for _, instance := range instances {
				if instance.ConnectionDomain == "" || instance.Port == 0 {
					continue
				}
				regionID := instance.RegionID
				if regionID == "" {
					regionID = region
				}
				resources = append(resources, core.Resource{
					Provider:     core.ProviderAliyun,
					AccountID:    accountID,
					AccountName:  account.Name,
					RegionID:     regionID,
					ResourceID:   instance.InstanceID,
					ResourceName: instance.InstanceName,
					ResourceType: redisResourceType,
					Engine:       core.EngineRedis,
					Address:      instance.ConnectionDomain,
					Port:         instance.Port,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}

func (s *RedisSource) identityKey(account AccountConfig) (identity.Key, error) {
	credentials, err := account.ResolveCredentials()
	if err != nil {
		return identity.Key{}, err
	}
	return identity.Key{
		Provider:    core.ProviderAliyun,
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
