package aws

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	awslib "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	docdbsvc "github.com/aws/aws-sdk-go-v2/service/docdb"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elasticachesvc "github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	rdssvc "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	stssvc "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

type SDKClient struct {
	region      string
	httpClient  *http.Client
	sts         *stssvc.Client
	ec2         *ec2svc.Client
	elasticache *elasticachesvc.Client
	rds         *rdssvc.Client
	docdb       *docdbsvc.Client
}

func NewRedisSDKClient(account AccountConfig, region string, opts ClientOptions) (RedisClient, error) {
	return newSDKClient(account, region, opts)
}

func NewRDSSDKClient(account AccountConfig, region string, opts ClientOptions) (RDSClient, error) {
	return newSDKClient(account, region, opts)
}

func NewMongoSDKClient(account AccountConfig, region string, opts ClientOptions) (MongoClient, error) {
	return newSDKClient(account, region, opts)
}

func NewEC2SDKClient(account AccountConfig, region string, opts ClientOptions) (EC2Client, error) {
	return newSDKClient(account, region, opts)
}

func newSDKClient(account AccountConfig, region string, opts ClientOptions) (*SDKClient, error) {
	resolved, err := account.ResolveCredentials()
	if err != nil {
		return nil, err
	}
	var httpClient *http.Client
	cfg := awslib.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			resolved.AccessKeyID,
			resolved.SecretAccessKey,
			resolved.SessionToken,
		),
	}
	if opts.RequestTimeout > 0 {
		httpClient = &http.Client{Timeout: opts.RequestTimeout}
		cfg.HTTPClient = httpClient
	}
	return &SDKClient{
		region:      region,
		httpClient:  httpClient,
		sts:         stssvc.NewFromConfig(cfg),
		ec2:         ec2svc.NewFromConfig(cfg),
		elasticache: elasticachesvc.NewFromConfig(cfg),
		rds:         rdssvc.NewFromConfig(cfg),
		docdb:       docdbsvc.NewFromConfig(cfg),
	}, nil
}

func (c *SDKClient) GetCallerIdentity(ctx context.Context) (string, error) {
	output, err := c.sts.GetCallerIdentity(ctx, &stssvc.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if awslib.ToString(output.Account) == "" {
		return "", fmt.Errorf("sts GetCallerIdentity returned empty account id")
	}
	return awslib.ToString(output.Account), nil
}

func (c *SDKClient) DescribeRedisInstances(ctx context.Context) ([]RedisInstance, error) {
	var instances []RedisInstance
	paginator := elasticachesvc.NewDescribeReplicationGroupsPaginator(
		c.elasticache,
		&elasticachesvc.DescribeReplicationGroupsInput{},
	)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, group := range output.ReplicationGroups {
			endpoint := redisEndpoint(group)
			if endpoint == nil {
				continue
			}
			tags, err := c.elasticacheTags(ctx, awslib.ToString(group.ARN))
			tags, err = tagsOrEmptyUnlessContextDone(ctx, tags, err)
			if err != nil {
				return nil, err
			}
			instances = append(instances, RedisInstance{
				ID:      awslib.ToString(group.ReplicationGroupId),
				Name:    awslib.ToString(group.ReplicationGroupId),
				Region:  c.region,
				Address: awslib.ToString(endpoint.Address),
				Port:    int(awslib.ToInt32(endpoint.Port)),
				Tags:    tags,
			})
		}
	}
	return instances, nil
}

func redisEndpoint(group elasticachetypes.ReplicationGroup) *elasticachetypes.Endpoint {
	if group.ConfigurationEndpoint != nil && awslib.ToString(group.ConfigurationEndpoint.Address) != "" {
		return group.ConfigurationEndpoint
	}
	for _, nodeGroup := range group.NodeGroups {
		if nodeGroup.PrimaryEndpoint != nil && awslib.ToString(nodeGroup.PrimaryEndpoint.Address) != "" {
			return nodeGroup.PrimaryEndpoint
		}
	}
	return nil
}

func (c *SDKClient) DescribeRDSInstances(ctx context.Context) ([]RDSInstance, error) {
	var instances []RDSInstance
	paginator := rdssvc.NewDescribeDBInstancesPaginator(
		c.rds,
		&rdssvc.DescribeDBInstancesInput{},
	)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, db := range output.DBInstances {
			engine := awslib.ToString(db.Engine)
			if !isSupportedRDSEngine(engine) || db.Endpoint == nil {
				continue
			}
			instances = append(instances, RDSInstance{
				ID:      awslib.ToString(db.DBInstanceIdentifier),
				Name:    awslib.ToString(db.DBInstanceIdentifier),
				Region:  c.region,
				Engine:  rdsEngine(engine),
				Address: awslib.ToString(db.Endpoint.Address),
				Port:    int(awslib.ToInt32(db.Endpoint.Port)),
				Tags:    awsRDSTagsToMap(db.TagList),
			})
		}
	}
	return instances, nil
}

func isSupportedRDSEngine(engine string) bool {
	return engine == "mysql" || engine == "postgres" || strings.HasPrefix(engine, "aurora-mysql") || strings.HasPrefix(engine, "aurora-postgresql")
}

func rdsEngine(engine string) core.Engine {
	if strings.Contains(engine, "postgres") {
		return core.EnginePostgres
	}
	return core.EngineMySQL
}

func (c *SDKClient) DescribeMongoInstances(ctx context.Context) ([]MongoInstance, error) {
	var instances []MongoInstance
	paginator := docdbsvc.NewDescribeDBClustersPaginator(
		c.docdb,
		&docdbsvc.DescribeDBClustersInput{},
	)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, cluster := range output.DBClusters {
			if awslib.ToString(cluster.Endpoint) == "" || awslib.ToInt32(cluster.Port) == 0 {
				continue
			}
			tags, err := c.docdbTags(ctx, awslib.ToString(cluster.DBClusterArn))
			tags, err = tagsOrEmptyUnlessContextDone(ctx, tags, err)
			if err != nil {
				return nil, err
			}
			instances = append(instances, MongoInstance{
				ID:      awslib.ToString(cluster.DBClusterIdentifier),
				Name:    awslib.ToString(cluster.DBClusterIdentifier),
				Region:  c.region,
				Address: awslib.ToString(cluster.Endpoint),
				Port:    int(awslib.ToInt32(cluster.Port)),
				Tags:    tags,
			})
		}
	}
	return instances, nil
}

func (c *SDKClient) DescribeEC2Instances(ctx context.Context) ([]EC2Instance, error) {
	var instances []EC2Instance
	paginator := ec2svc.NewDescribeInstancesPaginator(
		c.ec2,
		&ec2svc.DescribeInstancesInput{},
	)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				tags := ec2TagsToMap(instance.Tags)
				instances = append(instances, EC2Instance{
					ID:        awslib.ToString(instance.InstanceId),
					Name:      firstNonEmpty(tags["Name"], awslib.ToString(instance.InstanceId)),
					Region:    c.region,
					PrivateIP: awslib.ToString(instance.PrivateIpAddress),
					PublicIP:  awslib.ToString(instance.PublicIpAddress),
					Tags:      tags,
				})
			}
		}
	}
	return instances, nil
}

func (c *SDKClient) elasticacheTags(ctx context.Context, arn string) (map[string]string, error) {
	if arn == "" {
		return map[string]string{}, nil
	}
	output, err := c.elasticache.ListTagsForResource(ctx, &elasticachesvc.ListTagsForResourceInput{
		ResourceName: awslib.String(arn),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(output.TagList))
	for _, tag := range output.TagList {
		out[awslib.ToString(tag.Key)] = awslib.ToString(tag.Value)
	}
	return out, nil
}

func (c *SDKClient) docdbTags(ctx context.Context, arn string) (map[string]string, error) {
	if arn == "" {
		return map[string]string{}, nil
	}
	output, err := c.docdb.ListTagsForResource(ctx, &docdbsvc.ListTagsForResourceInput{
		ResourceName: awslib.String(arn),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(output.TagList))
	for _, tag := range output.TagList {
		out[awslib.ToString(tag.Key)] = awslib.ToString(tag.Value)
	}
	return out, nil
}

func tagsOrEmptyUnlessContextDone(ctx context.Context, tags map[string]string, err error) (map[string]string, error) {
	if err == nil {
		return tags, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return map[string]string{}, nil
}

func awsRDSTagsToMap(tags []rdstypes.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := awslib.ToString(tag.Key)
		if key == "" {
			continue
		}
		out[key] = awslib.ToString(tag.Value)
	}
	return out
}

func ec2TagsToMap(tags []ec2types.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := awslib.ToString(tag.Key)
		if key == "" {
			continue
		}
		out[key] = awslib.ToString(tag.Value)
	}
	return out
}
