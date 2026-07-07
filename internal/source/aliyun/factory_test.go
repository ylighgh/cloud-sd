package aliyun

import (
	"testing"
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/identity"
)

func TestFactoryBuildsEnabledAliyunSources(t *testing.T) {
	factory := NewFactory([]AccountConfig{{
		Name:            "prod",
		Regions:         []string{"cn-hangzhou"},
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	}}, 15*time.Second, identity.NewMemoryCache())

	sources := factory.BuildSources(core.EngineSet{
		core.EngineRedis:    true,
		core.EngineMySQL:    true,
		core.EnginePostgres: true,
		core.EngineMongo:    true,
		core.EngineNode:     true,
	})

	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.Name())
	}
	want := []string{
		"aliyun-redis",
		"aliyun-rds-mysql",
		"aliyun-rds-postgres",
		"aliyun-mongo",
		"aliyun-ecs-node",
	}
	if len(names) != len(want) {
		t.Fatalf("source names = %#v, want %#v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("source names = %#v, want %#v", names, want)
		}
	}
}
