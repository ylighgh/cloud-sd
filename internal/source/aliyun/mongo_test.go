package aliyun

import (
	"context"
	"testing"

	"github.com/ylighgh/cloud-sd/internal/core"
)

func TestMongoSourceConvertsAliyunMongoInstancesToResources(t *testing.T) {
	source := NewMongoSource([]AccountConfig{
		{
			Name:            "prod",
			Regions:         []string{"cn-hangzhou"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	}, WithMongoClientFactory(func(account AccountConfig, region string, opts ClientOptions) (MongoClient, error) {
		return &fakeMongoClient{
			accountID: "123456789",
			instances: []MongoInstance{
				{
					InstanceID:       "dds-bp123",
					InstanceName:     "prod-mongo",
					RegionID:         "cn-hangzhou",
					ConnectionString: "mongo.example.com",
					Port:             3717,
					Tags: map[string]string{
						"cloud_sd_scope": "id1",
					},
				},
			},
		}, nil
	}))

	if source.Name() != "aliyun-mongo" {
		t.Fatalf("Name() = %q", source.Name())
	}
	if source.Provider() != core.ProviderAliyun {
		t.Fatalf("Provider() = %q", source.Provider())
	}

	resources, err := source.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources len = %d", len(resources))
	}

	got := resources[0]
	if got.Provider != core.ProviderAliyun {
		t.Fatalf("provider = %q", got.Provider)
	}
	if got.AccountID != "123456789" {
		t.Fatalf("account ID = %q", got.AccountID)
	}
	if got.AccountName != "prod" {
		t.Fatalf("account name = %q", got.AccountName)
	}
	if got.RegionID != "cn-hangzhou" {
		t.Fatalf("region ID = %q", got.RegionID)
	}
	if got.ResourceID != "dds-bp123" {
		t.Fatalf("resource ID = %q", got.ResourceID)
	}
	if got.ResourceName != "prod-mongo" {
		t.Fatalf("resource name = %q", got.ResourceName)
	}
	if got.ResourceType != "mongodb_instance" {
		t.Fatalf("resource type = %q", got.ResourceType)
	}
	if got.Engine != core.EngineMongo {
		t.Fatalf("engine = %q", got.Engine)
	}
	if got.Address != "mongo.example.com" || got.Port != 3717 {
		t.Fatalf("endpoint = %s:%d", got.Address, got.Port)
	}
	if got.Tags["cloud_sd_scope"] != "id1" {
		t.Fatalf("tags = %#v", got.Tags)
	}
}

type fakeMongoClient struct {
	accountID string
	instances []MongoInstance
}

func (f *fakeMongoClient) GetCallerIdentity(context.Context) (string, error) {
	return f.accountID, nil
}

func (f *fakeMongoClient) DescribeMongoInstances(context.Context) ([]MongoInstance, error) {
	return f.instances, nil
}
