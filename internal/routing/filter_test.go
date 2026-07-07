package routing

import (
	"testing"

	"github.com/ylighgh/cloud-sd/internal/core"
)

func TestFilterKeepsRedisResourcesWithMatchingScope(t *testing.T) {
	resources := []core.Resource{
		{
			Provider:   core.ProviderAliyun,
			AccountID:  "123456789",
			RegionID:   "cn-hangzhou",
			ResourceID: "r-keep",
			Engine:     core.EngineRedis,
			Address:    "redis.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_scope":   "id1",
				"cloud_sd_disable": "false",
			},
		},
		{
			ResourceID: "r-wrong-scope",
			Engine:     core.EngineRedis,
			Address:    "wrong-scope.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_scope": "other",
			},
		},
		{
			ResourceID: "r-disabled",
			Engine:     core.EngineRedis,
			Address:    "disabled.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_scope":   "id1",
				"cloud_sd_disable": "true",
			},
		},
		{
			ResourceID: "pg-ignore",
			Engine:     core.EnginePostgres,
			Address:    "postgres.example.com",
			Port:       5432,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
			},
		},
	}

	filtered := Filter(resources, Rules{
		Engine:     core.EngineRedis,
		Scopes:     []string{"id1"},
		ScopeTag:   "cloud_sd_scope",
		DisableTag: "cloud_sd_disable",
	})

	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1: %#v", len(filtered), filtered)
	}
	if filtered[0].ResourceID != "r-keep" {
		t.Fatalf("kept resource = %q", filtered[0].ResourceID)
	}
}

func TestFilterWithEmptyScopesKeepsAllMatchingEngineResources(t *testing.T) {
	resources := []core.Resource{
		{
			ResourceID: "r-without-scope",
			Engine:     core.EngineRedis,
			Address:    "redis-no-scope.example.com",
			Port:       6379,
			Tags:       map[string]string{},
		},
		{
			ResourceID: "r-other-scope",
			Engine:     core.EngineRedis,
			Address:    "redis-other-scope.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_scope": "other",
			},
		},
		{
			ResourceID: "r-disabled",
			Engine:     core.EngineRedis,
			Address:    "disabled.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_disable": "true",
			},
		},
		{
			ResourceID: "pg-ignore",
			Engine:     core.EnginePostgres,
			Address:    "postgres.example.com",
			Port:       5432,
			Tags:       map[string]string{},
		},
	}

	filtered := Filter(resources, Rules{
		Engine:     core.EngineRedis,
		Scopes:     nil,
		ScopeTag:   "cloud_sd_scope",
		DisableTag: "cloud_sd_disable",
	})

	if len(filtered) != 2 {
		t.Fatalf("filtered len = %d, want 2: %#v", len(filtered), filtered)
	}
	if filtered[0].ResourceID != "r-without-scope" {
		t.Fatalf("first kept resource = %q", filtered[0].ResourceID)
	}
	if filtered[1].ResourceID != "r-other-scope" {
		t.Fatalf("second kept resource = %q", filtered[1].ResourceID)
	}
}
