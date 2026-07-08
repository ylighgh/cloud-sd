package identity

import (
	"testing"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

func TestMemoryCacheStoresIdentityByProviderAccountNameAndAccessKey(t *testing.T) {
	cache := NewMemoryCache()
	key := Key{
		Provider:    core.ProviderAliyun,
		AccountName: "prod",
		AccessKeyID: "ak-1",
	}
	value := Identity{
		Provider:  core.ProviderAliyun,
		AccountID: "123456789",
	}

	cache.Set(key, value)

	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("Get() ok = false")
	}
	if got.AccountID != "123456789" {
		t.Fatalf("account ID = %q", got.AccountID)
	}

	if _, ok := cache.Get(Key{Provider: core.ProviderAliyun, AccountName: "prod", AccessKeyID: "ak-2"}); ok {
		t.Fatal("Get() for a different access key returned a cached identity")
	}
}
