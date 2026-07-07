package sd

import (
	"testing"

	"github.com/ylighgh/cloud-sd/internal/core"
)

func TestBuildTargetGroupsProducesPrometheusHTTPSDFormat(t *testing.T) {
	groups := BuildTargetGroups([]core.Resource{
		{
			Provider:     core.ProviderAliyun,
			AccountID:    "123456789",
			AccountName:  "prod",
			RegionID:     "cn-hangzhou",
			ResourceID:   "r-bp123",
			ResourceName: "prod-redis-cache",
			ResourceType: "redis_instance",
			Engine:       core.EngineRedis,
			Address:      "redis.example.com",
			Port:         6379,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
				"env":            "prod",
			},
			Labels: map[string]string{
				"team": "sre",
			},
		},
	}, Options{ScopeTag: "cloud_sd_scope"})

	if len(groups) != 1 {
		t.Fatalf("groups len = %d", len(groups))
	}
	if got := groups[0].Targets; len(got) != 1 || got[0] != "redis.example.com:6379" {
		t.Fatalf("targets = %#v", got)
	}

	labels := groups[0].Labels
	assertLabel(t, labels, "vendor", "aliyun")
	assertLabel(t, labels, "account", "prod")
	assertLabel(t, labels, "account_id", "123456789")
	assertLabel(t, labels, "region", "cn-hangzhou")
	assertLabel(t, labels, "group", "id1")
	assertLabel(t, labels, "name", "prod-redis-cache")
	assertLabel(t, labels, "iid", "r-bp123")
	assertLabel(t, labels, "cservice", "redis")
	assertLabel(t, labels, "resource_type", "redis_instance")
	assertLabel(t, labels, "engine", "redis")
	assertLabel(t, labels, "team", "sre")

	assertLabelMissing(t, labels, "provider")
	assertLabelMissing(t, labels, "account_name")
	assertLabelMissing(t, labels, "region_id")
	assertLabelMissing(t, labels, "resource_id")
	assertLabelMissing(t, labels, "resource_name")
	assertLabelMissing(t, labels, "scope")
}

func assertLabel(t *testing.T, labels map[string]string, key, want string) {
	t.Helper()
	if got := labels[key]; got != want {
		t.Fatalf("label %s = %q, want %q", key, got, want)
	}
}

func assertLabelMissing(t *testing.T, labels map[string]string, key string) {
	t.Helper()
	if got, ok := labels[key]; ok {
		t.Fatalf("label %s = %q, want missing", key, got)
	}
}
