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
	ec2ResourceType        = "ec2_instance"
	nodeExporterTargetPort = 9100
)

type EC2Client interface {
	GetCallerIdentity(ctx context.Context) (string, error)
	DescribeEC2Instances(ctx context.Context) ([]EC2Instance, error)
}

type EC2ClientFactory func(account AccountConfig, region string, opts ClientOptions) (EC2Client, error)

type EC2Instance struct {
	ID        string
	Name      string
	Region    string
	PrivateIP string
	PublicIP  string
	Tags      map[string]string
}

type NodeSource struct {
	accounts       []AccountConfig
	clientFactory  EC2ClientFactory
	requestTimeout time.Duration
	identityCache  identity.Cache
}

type NodeOption func(*NodeSource)

func WithEC2ClientFactory(factory EC2ClientFactory) NodeOption {
	return func(source *NodeSource) {
		source.clientFactory = factory
	}
}

func WithEC2RequestTimeout(timeout time.Duration) NodeOption {
	return func(source *NodeSource) {
		source.requestTimeout = timeout
	}
}

func WithEC2IdentityCache(cache identity.Cache) NodeOption {
	return func(source *NodeSource) {
		source.identityCache = cache
	}
}

func NewNodeSource(accounts []AccountConfig, opts ...NodeOption) *NodeSource {
	source := &NodeSource{
		accounts:       append([]AccountConfig(nil), accounts...),
		clientFactory:  NewEC2SDKClient,
		requestTimeout: 20 * time.Second,
		identityCache:  identity.NewMemoryCache(),
	}
	for _, opt := range opts {
		opt(source)
	}
	return source
}

func (s *NodeSource) Name() string {
	return "aws-ec2-node"
}

func (s *NodeSource) Provider() core.Provider {
	return core.ProviderAWS
}

func (s *NodeSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, account := range s.accounts {
		for _, region := range account.Regions {
			client, err := s.clientFactory(account, region, ClientOptions{RequestTimeout: s.requestTimeout})
			if err != nil {
				errs = append(errs, fmt.Errorf("create aws EC2 client for account %q region %q: %w", account.Name, region, err))
				continue
			}
			accountID, err := resolveAccountID(ctx, s.identityCache, account, client)
			if err != nil {
				errs = append(errs, fmt.Errorf("resolve aws account identity for account %q region %q: %w", account.Name, region, err))
				continue
			}
			instances, err := client.DescribeEC2Instances(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("describe aws EC2 instances for account %q region %q: %w", account.Name, region, err))
				continue
			}
			for _, instance := range instances {
				address := firstNonEmpty(instance.PrivateIP, instance.PublicIP)
				if address == "" {
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
					ResourceType: ec2ResourceType,
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
