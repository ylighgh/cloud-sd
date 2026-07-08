package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
)

const redisResourceType = "elasticache_redis_instance"

type RedisClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeRedisInstances(ctx context.Context) ([]RedisInstance, error)
}

type RedisClientFactory func(account AccountConfig, region string, opts ClientOptions) (RedisClient, error)

type RedisInstance struct {
	ID      string
	Name    string
	Region  string
	Address string
	Port    int
	Tags    map[string]string
}

type RedisSource struct {
	accounts       []AccountConfig
	clientFactory  RedisClientFactory
	requestTimeout time.Duration
	identityCache  identity.Cache
}

type RedisOption func(*RedisSource)

func WithRedisClientFactory(factory RedisClientFactory) RedisOption {
	return func(source *RedisSource) {
		source.clientFactory = factory
	}
}

func WithRedisRequestTimeout(timeout time.Duration) RedisOption {
	return func(source *RedisSource) {
		source.requestTimeout = timeout
	}
}

func WithRedisIdentityCache(cache identity.Cache) RedisOption {
	return func(source *RedisSource) {
		source.identityCache = cache
	}
}

func NewRedisSource(accounts []AccountConfig, opts ...RedisOption) *RedisSource {
	source := &RedisSource{
		accounts:       append([]AccountConfig(nil), accounts...),
		clientFactory:  NewRedisSDKClient,
		requestTimeout: 20 * time.Second,
		identityCache:  identity.NewMemoryCache(),
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

func (s *RedisSource) Name() string {
	return "aws-elasticache-redis"
}

func (s *RedisSource) Provider() core.Provider {
	return core.ProviderAWS
}

func (s *RedisSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aws ElastiCache client for account %q region %q: %w", account.Name, region, err))
				continue
			}
			accountID, err := resolveAccountID(ctx, s.identityCache, account, client)
			if err != nil {
				errs = append(errs, fmt.Errorf("resolve aws account identity for account %q region %q: %w", account.Name, region, err))
				continue
			}
			instances, err := client.DescribeRedisInstances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aws ElastiCache Redis resources for account %q region %q: %w", account.Name, region, err))
				continue
			}
			for _, instance := range instances {
				if instance.Address == "" || instance.Port == 0 {
					continue
				}
				regionID := firstNonEmpty(instance.Region, region)
				resources = append(resources, core.Resource{
					Provider:     core.ProviderAWS,
					AccountID:    accountID,
					AccountName:  account.Name,
					RegionID:     regionID,
					ResourceID:   instance.ID,
					ResourceName: firstNonEmpty(instance.Name, instance.ID),
					ResourceType: redisResourceType,
					Engine:       core.EngineRedis,
					Address:      instance.Address,
					Port:         instance.Port,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}
