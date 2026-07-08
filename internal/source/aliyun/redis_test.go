package aliyun

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
)

func TestRedisSourceConvertsAliyunInstancesToResources(t *testing.T) {
	source := NewRedisSource([]AccountConfig{
		{
			Name:            "prod",
			Regions:         []string{"cn-hangzhou"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	}, WithClientFactory(func(account AccountConfig, region string, opts ClientOptions) (CloudClient, error) {
		return &fakeCloudClient{
			accountID: "123456789",
			instances: []RedisInstance{
				{
					InstanceID:       "r-bp123",
					InstanceName:     "prod-redis-cache",
					RegionID:         "cn-hangzhou",
					ConnectionDomain: "redis.example.com",
					Port:             6379,
					Tags: map[string]string{
						"cloud_sd_scope": "id1",
					},
				},
			},
		}, nil
	}))

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
	if got.ResourceID != "r-bp123" {
		t.Fatalf("resource ID = %q", got.ResourceID)
	}
	if got.ResourceName != "prod-redis-cache" {
		t.Fatalf("resource name = %q", got.ResourceName)
	}
	if got.Engine != core.EngineRedis {
		t.Fatalf("engine = %q", got.Engine)
	}
	if got.Address != "redis.example.com" || got.Port != 6379 {
		t.Fatalf("endpoint = %s:%d", got.Address, got.Port)
	}
}

func TestRedisSourceExposesSourceMetadata(t *testing.T) {
	source := NewRedisSource([]AccountConfig{{Name: "prod", Regions: []string{"cn-hangzhou"}}})

	if source.Name() != "aliyun-redis" {
		t.Fatalf("Name() = %q", source.Name())
	}
	if source.Provider() != core.ProviderAliyun {
		t.Fatalf("Provider() = %q", source.Provider())
	}
}

func TestRedisSourceCachesAccountIdentityAcrossRefreshes(t *testing.T) {
	identityCalls := 0
	source := NewRedisSource([]AccountConfig{
		{
			Name:            "prod",
			Regions:         []string{"cn-hangzhou"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	}, WithClientFactory(func(account AccountConfig, region string, opts ClientOptions) (CloudClient, error) {
		return &fakeCloudClient{
			accountID:     "123456789",
			identityHook:  func() { identityCalls++ },
			requestConfig: opts,
			instances: []RedisInstance{
				{
					InstanceID:       "r-bp123",
					RegionID:         "cn-hangzhou",
					ConnectionDomain: "redis.example.com",
					Port:             6379,
				},
			},
		}, nil
	}))

	if _, err := source.ListResources(context.Background()); err != nil {
		t.Fatalf("first ListResources(): %v", err)
	}
	if _, err := source.ListResources(context.Background()); err != nil {
		t.Fatalf("second ListResources(): %v", err)
	}
	if identityCalls != 1 {
		t.Fatalf("identity calls = %d, want 1", identityCalls)
	}
}

func TestRedisSourceUsesSharedIdentityCacheAcrossSources(t *testing.T) {
	identityCalls := 0
	cache := identity.NewMemoryCache()
	makeSource := func() *RedisSource {
		return NewRedisSource([]AccountConfig{
			{
				Name:            "prod",
				Regions:         []string{"cn-hangzhou"},
				AccessKeyID:     "ak",
				AccessKeySecret: "sk",
			},
		}, WithIdentityCache(cache), WithClientFactory(func(account AccountConfig, region string, opts ClientOptions) (CloudClient, error) {
			return &fakeCloudClient{
				accountID:    "123456789",
				identityHook: func() { identityCalls++ },
				instances: []RedisInstance{
					{
						InstanceID:       "r-bp123",
						RegionID:         "cn-hangzhou",
						ConnectionDomain: "redis.example.com",
						Port:             6379,
					},
				},
			}, nil
		}))
	}

	if _, err := makeSource().ListResources(context.Background()); err != nil {
		t.Fatalf("first source ListResources(): %v", err)
	}
	if _, err := makeSource().ListResources(context.Background()); err != nil {
		t.Fatalf("second source ListResources(): %v", err)
	}
	if identityCalls != 1 {
		t.Fatalf("identity calls = %d, want 1", identityCalls)
	}
}

func TestRedisSourceReturnsPartialResourcesWhenOneAccountFails(t *testing.T) {
	source := NewRedisSource([]AccountConfig{
		{
			Name:            "prod",
			Regions:         []string{"cn-hangzhou"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
		{
			Name:            "game",
			Regions:         []string{"cn-shanghai"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	}, WithClientFactory(func(account AccountConfig, region string, opts ClientOptions) (CloudClient, error) {
		if account.Name == "game" {
			return &fakeCloudClient{err: errors.New("api unavailable")}, nil
		}
		return &fakeCloudClient{
			accountID: "123456789",
			instances: []RedisInstance{
				{
					InstanceID:       "r-bp123",
					RegionID:         "cn-hangzhou",
					ConnectionDomain: "redis.example.com",
					Port:             6379,
				},
			},
		}, nil
	}))

	resources, err := source.ListResources(context.Background())
	if err == nil {
		t.Fatal("ListResources() error = nil")
	}
	if len(resources) != 1 || resources[0].ResourceID != "r-bp123" {
		t.Fatalf("resources = %#v, want partial successful resources", resources)
	}
}

func TestRedisSourcePassesRequestTimeoutToClientFactory(t *testing.T) {
	var got ClientOptions
	source := NewRedisSource([]AccountConfig{
		{
			Name:            "prod",
			Regions:         []string{"cn-hangzhou"},
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	}, WithRequestTimeout(30*time.Second), WithClientFactory(func(account AccountConfig, region string, opts ClientOptions) (CloudClient, error) {
		got = opts
		return &fakeCloudClient{accountID: "123456789"}, nil
	}))

	if _, err := source.ListResources(context.Background()); err != nil {
		t.Fatalf("ListResources(): %v", err)
	}
	if got.RequestTimeout != 30*time.Second {
		t.Fatalf("request timeout = %s, want 30s", got.RequestTimeout)
	}
}

type fakeCloudClient struct {
	accountID     string
	instances     []RedisInstance
	err           error
	identityHook  func()
	requestConfig ClientOptions
}

func (f *fakeCloudClient) GetCallerIdentity(context.Context) (string, error) {
	if f.identityHook != nil {
		f.identityHook()
	}
	if f.err != nil {
		return "", f.err
	}
	return f.accountID, nil
}

func (f *fakeCloudClient) DescribeRedisInstances(context.Context) ([]RedisInstance, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.instances, nil
}
