package aliyun

import (
	"context"
	"testing"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

func TestNodeSourceConvertsAliyunECSInstancesToNodeResources(t *testing.T) {
	source := NewNodeSource([]AccountConfig{
		{
			Name:            "prod",
			Regions:         []string{"cn-hangzhou"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	}, WithECSClientFactory(func(account AccountConfig, region string, opts ClientOptions) (ECSClient, error) {
		return &fakeECSClient{
			accountID: "123456789",
			instances: []ECSInstance{
				{
					InstanceID:   "i-private",
					InstanceName: "prod-node-private",
					RegionID:     "cn-hangzhou",
					PrivateIP:    "10.0.1.10",
					PublicIP:     "203.0.113.10",
					Tags: map[string]string{
						"cloud_sd_scope": "id1",
					},
				},
				{
					InstanceID:   "i-public",
					InstanceName: "prod-node-public",
					RegionID:     "cn-hangzhou",
					PublicIP:     "203.0.113.20",
					Tags: map[string]string{
						"cloud_sd_scope": "id2",
					},
				},
				{
					InstanceID:   "i-without-address",
					InstanceName: "skip-without-address",
					RegionID:     "cn-hangzhou",
				},
			},
		}, nil
	}))

	if source.Name() != "aliyun-ecs-node" {
		t.Fatalf("Name() = %q", source.Name())
	}
	if source.Provider() != core.ProviderAliyun {
		t.Fatalf("Provider() = %q", source.Provider())
	}

	resources, err := source.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("resources len = %d, want 2", len(resources))
	}

	privateNode := resources[0]
	if privateNode.Provider != core.ProviderAliyun {
		t.Fatalf("provider = %q", privateNode.Provider)
	}
	if privateNode.AccountID != "123456789" {
		t.Fatalf("account ID = %q", privateNode.AccountID)
	}
	if privateNode.AccountName != "prod" {
		t.Fatalf("account name = %q", privateNode.AccountName)
	}
	if privateNode.RegionID != "cn-hangzhou" {
		t.Fatalf("region ID = %q", privateNode.RegionID)
	}
	if privateNode.ResourceID != "i-private" {
		t.Fatalf("resource ID = %q", privateNode.ResourceID)
	}
	if privateNode.ResourceName != "prod-node-private" {
		t.Fatalf("resource name = %q", privateNode.ResourceName)
	}
	if privateNode.ResourceType != "ecs_instance" {
		t.Fatalf("resource type = %q", privateNode.ResourceType)
	}
	if privateNode.Engine != core.EngineNode {
		t.Fatalf("engine = %q", privateNode.Engine)
	}
	if privateNode.Address != "10.0.1.10" || privateNode.Port != 9100 {
		t.Fatalf("endpoint = %s:%d", privateNode.Address, privateNode.Port)
	}
	if privateNode.Tags["cloud_sd_scope"] != "id1" {
		t.Fatalf("tags = %#v", privateNode.Tags)
	}

	publicNode := resources[1]
	if publicNode.Address != "203.0.113.20" || publicNode.Port != 9100 {
		t.Fatalf("fallback endpoint = %s:%d", publicNode.Address, publicNode.Port)
	}
}

type fakeECSClient struct {
	accountID string
	instances []ECSInstance
}

func (f *fakeECSClient) GetCallerIdentity(context.Context) (string, error) {
	return f.accountID, nil
}

func (f *fakeECSClient) DescribeECSInstances(context.Context) ([]ECSInstance, error) {
	return f.instances, nil
}
