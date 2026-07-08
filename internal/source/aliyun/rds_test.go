package aliyun

import (
	"context"
	"testing"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

func TestRDSSourceConvertsMySQLAndPostgresInstancesToResources(t *testing.T) {
	tests := []struct {
		name             string
		source           func([]AccountConfig, ...RDSOption) *RDSSource
		wantSourceName   string
		wantAPIEngine    string
		wantEngine       core.Engine
		wantResourceType string
		instance         RDSInstance
		wantTarget       string
		wantPort         int
	}{
		{
			name:             "mysql",
			source:           NewMySQLSource,
			wantSourceName:   "aliyun-rds-mysql",
			wantAPIEngine:    "MySQL",
			wantEngine:       core.EngineMySQL,
			wantResourceType: "rds_mysql_instance",
			instance: RDSInstance{
				InstanceID:       "rm-mysql",
				InstanceName:     "prod-mysql",
				RegionID:         "cn-hangzhou",
				ConnectionString: "mysql.example.com",
				Port:             3306,
				Tags: map[string]string{
					"cloud_sd_scope": "id1",
				},
			},
			wantTarget: "mysql.example.com",
			wantPort:   3306,
		},
		{
			name:             "postgres",
			source:           NewPostgresSource,
			wantSourceName:   "aliyun-rds-postgres",
			wantAPIEngine:    "PostgreSQL",
			wantEngine:       core.EnginePostgres,
			wantResourceType: "rds_postgres_instance",
			instance: RDSInstance{
				InstanceID:       "pg-postgres",
				InstanceName:     "prod-postgres",
				RegionID:         "cn-shanghai",
				ConnectionString: "postgres.example.com",
				Port:             5432,
				Tags: map[string]string{
					"cloud_sd_scope": "game-id1",
				},
			},
			wantTarget: "postgres.example.com",
			wantPort:   5432,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAPIEngine string
			source := tt.source(
				[]AccountConfig{{
					Name:            "prod",
					Regions:         []string{tt.instance.RegionID},
					AccessKeyID:     "ak",
					AccessKeySecret: "sk",
				}},
				WithRDSClientFactory(func(account AccountConfig, region string, opts ClientOptions) (RDSClient, error) {
					return &fakeRDSClient{
						accountID: "123456789",
						instances: []RDSInstance{
							tt.instance,
						},
						describeHook: func(engine string) {
							gotAPIEngine = engine
						},
					}, nil
				}),
			)

			if source.Name() != tt.wantSourceName {
				t.Fatalf("Name() = %q, want %q", source.Name(), tt.wantSourceName)
			}
			if source.Provider() != core.ProviderAliyun {
				t.Fatalf("Provider() = %q", source.Provider())
			}

			resources, err := source.ListResources(context.Background())
			if err != nil {
				t.Fatalf("ListResources() error = %v", err)
			}
			if gotAPIEngine != tt.wantAPIEngine {
				t.Fatalf("api engine = %q, want %q", gotAPIEngine, tt.wantAPIEngine)
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
			if got.RegionID != tt.instance.RegionID {
				t.Fatalf("region ID = %q", got.RegionID)
			}
			if got.ResourceID != tt.instance.InstanceID {
				t.Fatalf("resource ID = %q", got.ResourceID)
			}
			if got.ResourceName != tt.instance.InstanceName {
				t.Fatalf("resource name = %q", got.ResourceName)
			}
			if got.ResourceType != tt.wantResourceType {
				t.Fatalf("resource type = %q, want %q", got.ResourceType, tt.wantResourceType)
			}
			if got.Engine != tt.wantEngine {
				t.Fatalf("engine = %q, want %q", got.Engine, tt.wantEngine)
			}
			if got.Address != tt.wantTarget || got.Port != tt.wantPort {
				t.Fatalf("endpoint = %s:%d", got.Address, got.Port)
			}
			if got.Tags["cloud_sd_scope"] != tt.instance.Tags["cloud_sd_scope"] {
				t.Fatalf("tags = %#v", got.Tags)
			}
		})
	}
}

type fakeRDSClient struct {
	accountID    string
	instances    []RDSInstance
	describeHook func(engine string)
}

func (f *fakeRDSClient) GetCallerIdentity(context.Context) (string, error) {
	return f.accountID, nil
}

func (f *fakeRDSClient) DescribeRDSInstances(_ context.Context, engine string) ([]RDSInstance, error) {
	if f.describeHook != nil {
		f.describeHook(engine)
	}
	return f.instances, nil
}
