package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/identity"
)

const mongoResourceType = "documentdb_instance"

type MongoClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeMongoInstances(ctx context.Context) ([]MongoInstance, error)
}

type MongoClientFactory func(account AccountConfig, region string, opts ClientOptions) (MongoClient, error)

type MongoInstance struct {
	ID      string
	Name    string
	Region  string
	Address string
	Port    int
	Tags    map[string]string
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
	return "aws-documentdb-mongo"
}

func (s *MongoSource) Provider() core.Provider {
	return core.ProviderAWS
}

func (s *MongoSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aws DocumentDB client for account %q region %q: %w", account.Name, region, err))
				continue
			}
			accountID, err := resolveAccountID(ctx, s.identityCache, account, client)
			if err != nil {
				errs = append(errs, fmt.Errorf("resolve aws account identity for account %q region %q: %w", account.Name, region, err))
				continue
			}
			instances, err := client.DescribeMongoInstances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aws DocumentDB resources for account %q region %q: %w", account.Name, region, err))
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
					ResourceType: mongoResourceType,
					Engine:       core.EngineMongo,
					Address:      instance.Address,
					Port:         instance.Port,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}
