package aws

import (
	"testing"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
)

func TestFactoryBuildsEnabledAWSSources(t *testing.T) {
	factory := NewFactory([]AccountConfig{{
		Name:            "prod",
		Regions:         []string{"ap-southeast-1"},
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
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
		"aws-elasticache-redis",
		"aws-rds-mysql",
		"aws-rds-postgres",
		"aws-documentdb-mongo",
		"aws-ec2-node",
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
