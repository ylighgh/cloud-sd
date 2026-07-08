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
	ecsResourceType        = "ecs_instance"
	nodeExporterTargetPort = 9100
)

type ECSClient interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeECSInstances(ctx context.Context) ([]ECSInstance, error)
}

type ECSClientFactory func(account AccountConfig, region string, opts ClientOptions) (ECSClient, error)

type ECSInstance struct {
	InstanceID   string
	InstanceName string
	RegionID     string
	PrivateIP    string
	PublicIP     string
	EIP          string
	Tags         map[string]string
}

type NodeSource struct {
	accounts       []AccountConfig
	clientFactory  ECSClientFactory
	requestTimeout time.Duration
	identityCache  identity.Cache
}

type NodeOption func(*NodeSource)

func WithECSClientFactory(factory ECSClientFactory) NodeOption {
	return func(source *NodeSource) {
		source.clientFactory = factory
	}
}

func WithECSRequestTimeout(timeout time.Duration) NodeOption {
	return func(source *NodeSource) {
		source.requestTimeout = timeout
	}
}

func WithECSIdentityCache(cache identity.Cache) NodeOption {
	return func(source *NodeSource) {
		source.identityCache = cache
	}
}

func NewNodeSource(accounts []AccountConfig, opts ...NodeOption) *NodeSource {
	source := &NodeSource{
		accounts:       append([]AccountConfig(nil), accounts...),
		clientFactory:  NewECSSDKClient,
		requestTimeout: 20 * time.Second,
		identityCache:  identity.NewMemoryCache(),
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

func (s *NodeSource) Name() string {
	return "aliyun-ecs-node"
}

func (s *NodeSource) Provider() core.Provider {
	return core.ProviderAliyun
}

func (s *NodeSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		if len(account.Regions) == 0 {
			continue
		}

		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aliyun ECS client for account %q region %q: %w", account.Name, region, err))
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

			instances, err := client.DescribeECSInstances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aliyun ECS instances for account %q region %q: %w", account.Name, region, err))
				continue
			}
			for _, instance := range instances {
				address := firstNonEmpty(instance.PrivateIP, instance.PublicIP, instance.EIP)
				if address == "" {
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
					ResourceType: ecsResourceType,
					Engine:       core.EngineNode,
					Address:      address,
					Port:         nodeExporterTargetPort,
					Tags:         copyMap(instance.Tags),
				})
			}
		}
	}
	return resources, errors.Join(errs...)
}

func (s *NodeSource) identityKey(account AccountConfig) (identity.Key, error) {
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
