package aws

import (
	"context"
	"testing"

	"github.com/ylighgh/cloud-sd/internal/core"
)

func TestRedisSourceConvertsElastiCacheResources(t *testing.T) {
	source := NewRedisSource(testAccounts(), WithRedisClientFactory(func(account AccountConfig, region string, opts ClientOptions) (RedisClient, error) {
		return &fakeRedisClient{accountID: "123456789012", instances: []RedisInstance{{
			ID:      "redis-prod",
			Name:    "redis-prod",
			Region:  "ap-southeast-1",
			Address: "redis.example.cache.amazonaws.com",
			Port:    6379,
			Tags:    map[string]string{"cloud_sd_scope": "id1"},
		}}}, nil
	}))

	resources, err := source.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	assertAWSResource(t, resources[0], core.EngineRedis, "elasticache_redis_instance", "redis.example.cache.amazonaws.com", 6379)
}

func TestRDSSourceConvertsMySQLAndPostgresResources(t *testing.T) {
	tests := []struct {
		name         string
		source       func([]AccountConfig, ...RDSOption) *RDSSource
		wantName     string
		wantEngine   core.Engine
		wantType     string
		instance     RDSInstance
		wantEndpoint string
		wantPort     int
	}{
		{
			name:       "mysql",
			source:     NewMySQLSource,
			wantName:   "aws-rds-mysql",
			wantEngine: core.EngineMySQL,
			wantType:   "rds_mysql_instance",
			instance: RDSInstance{
				ID:      "mysql-prod",
				Name:    "mysql-prod",
				Region:  "ap-southeast-1",
				Address: "mysql.example.rds.amazonaws.com",
				Port:    3306,
				Tags:    map[string]string{"cloud_sd_scope": "id1"},
			},
			wantEndpoint: "mysql.example.rds.amazonaws.com",
			wantPort:     3306,
		},
		{
			name:       "postgres",
			source:     NewPostgresSource,
			wantName:   "aws-rds-postgres",
			wantEngine: core.EnginePostgres,
			wantType:   "rds_postgres_instance",
			instance: RDSInstance{
				ID:      "postgres-prod",
				Name:    "postgres-prod",
				Region:  "ap-southeast-1",
				Address: "postgres.example.rds.amazonaws.com",
				Port:    5432,
				Tags:    map[string]string{"cloud_sd_scope": "id1"},
			},
			wantEndpoint: "postgres.example.rds.amazonaws.com",
			wantPort:     5432,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := tt.source(testAccounts(), WithRDSClientFactory(func(account AccountConfig, region string, opts ClientOptions) (RDSClient, error) {
				return &fakeRDSClient{accountID: "123456789012", instances: []RDSInstance{tt.instance}}, nil
			}))
			if source.Name() != tt.wantName {
				t.Fatalf("Name() = %q, want %q", source.Name(), tt.wantName)
			}

			resources, err := source.ListResources(context.Background())
			if err != nil {
				t.Fatalf("ListResources() error = %v", err)
			}
			assertAWSResource(t, resources[0], tt.wantEngine, tt.wantType, tt.wantEndpoint, tt.wantPort)
		})
	}
}

func TestRDSSourceFiltersResourcesByEngine(t *testing.T) {
	source := NewMySQLSource(testAccounts(), WithRDSClientFactory(func(account AccountConfig, region string, opts ClientOptions) (RDSClient, error) {
		return &fakeRDSClient{accountID: "123456789012", instances: []RDSInstance{
			{
				ID:      "mysql-prod",
				Name:    "mysql-prod",
				Region:  "ap-southeast-1",
				Engine:  core.EngineMySQL,
				Address: "mysql.example.rds.amazonaws.com",
				Port:    3306,
				Tags:    map[string]string{"cloud_sd_scope": "id1"},
			},
			{
				ID:      "postgres-prod",
				Name:    "postgres-prod",
				Region:  "ap-southeast-1",
				Engine:  core.EnginePostgres,
				Address: "postgres.example.rds.amazonaws.com",
				Port:    5432,
				Tags:    map[string]string{"cloud_sd_scope": "id1"},
			},
		}}, nil
	}))

	resources, err := source.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources len = %d, want 1", len(resources))
	}
	if resources[0].ResourceID != "mysql-prod" {
		t.Fatalf("resource ID = %q, want mysql-prod", resources[0].ResourceID)
	}
}

func TestMongoSourceConvertsDocumentDBResources(t *testing.T) {
	source := NewMongoSource(testAccounts(), WithMongoClientFactory(func(account AccountConfig, region string, opts ClientOptions) (MongoClient, error) {
		return &fakeMongoClient{accountID: "123456789012", instances: []MongoInstance{{
			ID:      "docdb-prod",
			Name:    "docdb-prod",
			Region:  "ap-southeast-1",
			Address: "docdb.example.docdb.amazonaws.com",
			Port:    27017,
			Tags:    map[string]string{"cloud_sd_scope": "id1"},
		}}}, nil
	}))

	resources, err := source.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	assertAWSResource(t, resources[0], core.EngineMongo, "documentdb_instance", "docdb.example.docdb.amazonaws.com", 27017)
}

func TestNodeSourceConvertsEC2Resources(t *testing.T) {
	source := NewNodeSource(testAccounts(), WithEC2ClientFactory(func(account AccountConfig, region string, opts ClientOptions) (EC2Client, error) {
		return &fakeEC2Client{accountID: "123456789012", instances: []EC2Instance{{
			ID:        "i-123",
			Name:      "prod-node",
			Region:    "ap-southeast-1",
			PrivateIP: "10.0.1.10",
			PublicIP:  "203.0.113.10",
			Tags:      map[string]string{"cloud_sd_scope": "id1"},
		}}}, nil
	}))

	resources, err := source.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	assertAWSResource(t, resources[0], core.EngineNode, "ec2_instance", "10.0.1.10", 9100)
}

func testAccounts() []AccountConfig {
	return []AccountConfig{{
		Name:            "prod",
		Regions:         []string{"ap-southeast-1"},
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	}}
}

func assertAWSResource(t *testing.T, resource core.Resource, engine core.Engine, resourceType, address string, port int) {
	t.Helper()
	if resource.Provider != core.ProviderAWS {
		t.Fatalf("provider = %q", resource.Provider)
	}
	if resource.AccountID != "123456789012" {
		t.Fatalf("account ID = %q", resource.AccountID)
	}
	if resource.AccountName != "prod" {
		t.Fatalf("account name = %q", resource.AccountName)
	}
	if resource.RegionID != "ap-southeast-1" {
		t.Fatalf("region = %q", resource.RegionID)
	}
	if resource.ResourceType != resourceType {
		t.Fatalf("resource type = %q, want %q", resource.ResourceType, resourceType)
	}
	if resource.Engine != engine {
		t.Fatalf("engine = %q, want %q", resource.Engine, engine)
	}
	if resource.Address != address || resource.Port != port {
		t.Fatalf("endpoint = %s:%d, want %s:%d", resource.Address, resource.Port, address, port)
	}
	if resource.Tags["cloud_sd_scope"] != "id1" {
		t.Fatalf("tags = %#v", resource.Tags)
	}
}

type fakeRedisClient struct {
	accountID string
	instances []RedisInstance
}

func (f *fakeRedisClient) GetCallerIdentity(context.Context) (string, error) {
	return f.accountID, nil
}

func (f *fakeRedisClient) DescribeRedisInstances(context.Context) ([]RedisInstance, error) {
	return f.instances, nil
}

type fakeRDSClient struct {
	accountID string
	instances []RDSInstance
}

func (f *fakeRDSClient) GetCallerIdentity(context.Context) (string, error) {
	return f.accountID, nil
}

func (f *fakeRDSClient) DescribeRDSInstances(context.Context) ([]RDSInstance, error) {
	return f.instances, nil
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

type fakeEC2Client struct {
	accountID string
	instances []EC2Instance
}

func (f *fakeEC2Client) GetCallerIdentity(context.Context) (string, error) {
	return f.accountID, nil
}

func (f *fakeEC2Client) DescribeEC2Instances(context.Context) ([]EC2Instance, error) {
	return f.instances, nil
}
