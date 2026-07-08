package aliyun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
)

const (
	rdsMySQLResourceType    = "rds_mysql_instance"
	rdsPostgresResourceType = "rds_postgres_instance"
)

type RDSClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeRDSInstances(ctx context.Context, engine string) ([]RDSInstance, error)
}

type RDSClientFactory func(account AccountConfig, region string, opts ClientOptions) (RDSClient, error)

type RDSInstance struct {
	InstanceID       string
	InstanceName     string
	RegionID         string
	ConnectionString string
	Port             int
	Engine           string
	Tags             map[string]string
}

type RDSSource struct {
	name           string
	engine         core.Engine
	apiEngine      string
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
	return newRDSSource("aliyun-rds-mysql", core.EngineMySQL, "MySQL", rdsMySQLResourceType, accounts, opts...)
}

func NewPostgresSource(accounts []AccountConfig, opts ...RDSOption) *RDSSource {
	return newRDSSource("aliyun-rds-postgres", core.EnginePostgres, "PostgreSQL", rdsPostgresResourceType, accounts, opts...)
}

func newRDSSource(name string, engine core.Engine, apiEngine, resourceType string, accounts []AccountConfig, opts ...RDSOption) *RDSSource {
	source := &RDSSource{
		name:           name,
		engine:         engine,
		apiEngine:      apiEngine,
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
	return core.ProviderAliyun
}

func (s *RDSSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		if len(account.Regions) == 0 {
			continue
		}

		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aliyun RDS client for account %q region %q: %w", account.Name, region, err))
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

			instances, err := client.DescribeRDSInstances(ctx, s.apiEngine)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aliyun RDS %s instances for account %q region %q: %w", s.apiEngine, account.Name, region, err))
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
					ResourceType: s.resourceType,
					Engine:       s.engine,
					Address:      instance.ConnectionString,
					Port:         instance.Port,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}

func (s *RDSSource) identityKey(account AccountConfig) (identity.Key, error) {
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
