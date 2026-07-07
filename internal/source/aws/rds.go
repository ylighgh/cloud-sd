package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/identity"
)

const (
	rdsMySQLResourceType    = "rds_mysql_instance"
	rdsPostgresResourceType = "rds_postgres_instance"
)

type RDSClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeRDSInstances(ctx context.Context) ([]RDSInstance, error)
}

type RDSClientFactory func(account AccountConfig, region string, opts ClientOptions) (RDSClient, error)

type RDSInstance struct {
	ID      string
	Name    string
	Region  string
	Engine  core.Engine
	Address string
	Port    int
	Tags    map[string]string
}

type RDSSource struct {
	name           string
	engine         core.Engine
	resourceType   string
	accounts       []AccountConfig
	clientFactory  RDSClientFactory
	requestTimeout time.Duration
	identityCache  identity.Cache
}

type RDSOption func(*RDSSource)

func WithRDSClientFactory(factory RDSClientFactory) RDSOption {
	return func(source *RDSSource) {
		source.clientFactory = factory
	}
}

func WithRDSRequestTimeout(timeout time.Duration) RDSOption {
	return func(source *RDSSource) {
		source.requestTimeout = timeout
	}
}

func WithRDSIdentityCache(cache identity.Cache) RDSOption {
	return func(source *RDSSource) {
		source.identityCache = cache
	}
}

func NewMySQLSource(accounts []AccountConfig, opts ...RDSOption) *RDSSource {
	return newRDSSource("aws-rds-mysql", core.EngineMySQL, rdsMySQLResourceType, accounts, opts...)
}

func NewPostgresSource(accounts []AccountConfig, opts ...RDSOption) *RDSSource {
	return newRDSSource("aws-rds-postgres", core.EnginePostgres, rdsPostgresResourceType, accounts, opts...)
}

func newRDSSource(name string, engine core.Engine, resourceType string, accounts []AccountConfig, opts ...RDSOption) *RDSSource {
	source := &RDSSource{
		name:           name,
		engine:         engine,
		resourceType:   resourceType,
		accounts:       append([]AccountConfig(nil), accounts...),
		clientFactory:  NewRDSSDKClient,
		requestTimeout: 20 * time.Second,
		identityCache:  identity.NewMemoryCache(),
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

func (s *RDSSource) Name() string {
	return s.name
}

func (s *RDSSource) Provider() core.Provider {
	return core.ProviderAWS
}

func (s *RDSSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aws RDS client for account %q region %q: %w", account.Name, region, err))
				continue
			}
			accountID, err := resolveAccountID(ctx, s.identityCache, account, client)
			if err != nil {
				errs = append(errs, fmt.Errorf("resolve aws account identity for account %q region %q: %w", account.Name, region, err))
				continue
			}
			instances, err := client.DescribeRDSInstances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aws RDS instances for account %q region %q: %w", account.Name, region, err))
				continue
			}
			for _, instance := range instances {
				if instance.Engine != "" && instance.Engine != s.engine {
					continue
				}
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
					ResourceType: s.resourceType,
					Engine:       s.engine,
					Address:      instance.Address,
					Port:         instance.Port,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}
