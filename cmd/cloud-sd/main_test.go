package main

import (
	"testing"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/config"
	"github.com/ylighgh/prometheus-cloud-sd/internal/identity"
	"github.com/ylighgh/prometheus-cloud-sd/internal/source/aliyun"
	awssource "github.com/ylighgh/prometheus-cloud-sd/internal/source/aws"
)

func TestBuildResourceSourcesUsesEnabledProviderFactories(t *testing.T) {
	cfg := config.Config{
		Collector: config.CollectorConfig{
			Engines: config.EngineConfig{
				Redis:    true,
				MySQL:    true,
				Postgres: true,
				Mongo:    true,
				Node:     true,
			},
			RequestTimeout: 20 * time.Second,
		},
		Aliyun: config.AliyunConfig{
			Enabled: true,
			Accounts: []aliyun.AccountConfig{{
				Name:            "aliyun-prod",
				Regions:         []string{"cn-hangzhou"},
				AccessKeyID:     "ak",
				AccessKeySecret: "sk",
			}},
		},
		AWS: config.AWSConfig{
			Enabled: true,
			Accounts: []awssource.AccountConfig{{
				Name:            "aws-prod",
				Regions:         []string{"ap-southeast-1"},
				AccessKeyID:     "ak",
				SecretAccessKey: "sk",
			}},
		},
	}

	sources := buildResourceSources(cfg, identity.NewMemoryCache())

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
