package aliyun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
)

const mongoResourceType = "mongodb_instance"

type MongoClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeMongoInstances(ctx context.Context) ([]MongoInstance, error)
}

type MongoClientFactory func(account AccountConfig, region string, opts ClientOptions) (MongoClient, error)

type MongoInstance struct {
	InstanceID       string
	InstanceName     string
	RegionID         string
	ConnectionString string
	Port             int
	Tags             map[string]string
}

type MongoSource struct {
	accounts       []AccountConfig
	clientFactory  MongoClientFactory
	requestTimeout time.Duration
	identityCache  identity.Cache
}

type MongoOption func(*MongoSource)

func WithMongoClientFactory(factory MongoClientFactory) MongoOption {
	return func(source *MongoSource) {
		source.clientFactory = factory
	}
}

func WithMongoRequestTimeout(timeout time.Duration) MongoOption {
	return func(source *MongoSource) {
		source.requestTimeout = timeout
	}
}

func WithMongoIdentityCache(cache identity.Cache) MongoOption {
	return func(source *MongoSource) {
		source.identityCache = cache
	}
}

func NewMongoSource(accounts []AccountConfig, opts ...MongoOption) *MongoSource {
	source := &MongoSource{
		accounts:       append([]AccountConfig(nil), accounts...),
		clientFactory:  NewMongoSDKClient,
		requestTimeout: 20 * time.Second,
		identityCache:  identity.NewMemoryCache(),
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

func (s *MongoSource) Name() string {
	return "aliyun-mongo"
}

func (s *MongoSource) Provider() core.Provider {
	return core.ProviderAliyun
}

func (s *MongoSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		if len(account.Regions) == 0 {
			continue
		}

		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aliyun MongoDB client for account %q region %q: %w", account.Name, region, err))
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

			instances, err := client.DescribeMongoInstances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aliyun MongoDB instances for account %q region %q: %w", account.Name, region, err))
				continue
			}
			for _, instance := range instances {
				if instance.ConnectionString == "" || instance.Port == 0 {
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
					ResourceType: mongoResourceType,
					Engine:       core.EngineMongo,
					Address:      instance.ConnectionString,
					Port:         instance.Port,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}

func (s *MongoSource) identityKey(account AccountConfig) (identity.Key, error) {
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
