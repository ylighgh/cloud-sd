package aws

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	awslib "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/docdb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/ylighgh/cloud-sd/internal/core"
)

type SDKClient struct {
	region      string
	sts         *sts.STS
	ec2         *ec2.EC2
	elasticache *elasticache.ElastiCache
	rds         *rds.RDS
	docdb       *docdb.DocDB
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
	cfg := &awslib.Config{
		Region: awslib.String(region),
		Credentials: credentials.NewStaticCredentials(
			resolved.AccessKeyID,
			resolved.SecretAccessKey,
			resolved.SessionToken,
		),
	}
	if opts.RequestTimeout > 0 {
		cfg.HTTPClient = &http.Client{Timeout: opts.RequestTimeout}
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("create aws session: %w", err)
	}
	return &SDKClient{
		region:      region,
		sts:         sts.New(sess),
		ec2:         ec2.New(sess),
		elasticache: elasticache.New(sess),
		rds:         rds.New(sess),
		docdb:       docdb.New(sess),
	}, nil
}

func (c *SDKClient) GetCallerIdentity(ctx context.Context) (string, error) {
	output, err := c.sts.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if awslib.StringValue(output.Account) == "" {
		return "", fmt.Errorf("sts GetCallerIdentity returned empty account id")
	}
	return awslib.StringValue(output.Account), nil
}

func (c *SDKClient) DescribeRedisInstances(ctx context.Context) ([]RedisInstance, error) {
	var instances []RedisInstance
	var pageErr error
	err := c.elasticache.DescribeReplicationGroupsPagesWithContext(
		ctx,
		&elasticache.DescribeReplicationGroupsInput{},
		func(output *elasticache.DescribeReplicationGroupsOutput, _ bool) bool {
			for _, group := range output.ReplicationGroups {
				endpoint := redisEndpoint(group)
				if endpoint == nil {
					continue
				}
				tags, err := c.elasticacheTags(ctx, awslib.StringValue(group.ARN))
				tags, err = tagsOrEmptyUnlessContextDone(ctx, tags, err)
				if err != nil {
					pageErr = err
					return false
				}
				instances = append(instances, RedisInstance{
					ID:      awslib.StringValue(group.ReplicationGroupId),
					Name:    awslib.StringValue(group.ReplicationGroupId),
					Region:  c.region,
					Address: awslib.StringValue(endpoint.Address),
					Port:    int(awslib.Int64Value(endpoint.Port)),
					Tags:    tags,
				})
			}
			return true
		},
	)
	if err != nil {
		return nil, err
	}
	if pageErr != nil {
		return nil, pageErr
	}
	return instances, nil
}

func redisEndpoint(group *elasticache.ReplicationGroup) *elasticache.Endpoint {
	if group == nil {
		return nil
	}
	if group.ConfigurationEndpoint != nil && awslib.StringValue(group.ConfigurationEndpoint.Address) != "" {
		return group.ConfigurationEndpoint
	}
	for _, nodeGroup := range group.NodeGroups {
		if nodeGroup.PrimaryEndpoint != nil && awslib.StringValue(nodeGroup.PrimaryEndpoint.Address) != "" {
			return nodeGroup.PrimaryEndpoint
		}
	}
	return nil
}

func (c *SDKClient) DescribeRDSInstances(ctx context.Context) ([]RDSInstance, error) {
	var instances []RDSInstance
	err := c.rds.DescribeDBInstancesPagesWithContext(
		ctx,
		&rds.DescribeDBInstancesInput{},
		func(output *rds.DescribeDBInstancesOutput, _ bool) bool {
			for _, db := range output.DBInstances {
				engine := awslib.StringValue(db.Engine)
				if !isSupportedRDSEngine(engine) || db.Endpoint == nil {
					continue
				}
				instances = append(instances, RDSInstance{
					ID:      awslib.StringValue(db.DBInstanceIdentifier),
					Name:    awslib.StringValue(db.DBInstanceIdentifier),
					Region:  c.region,
					Engine:  rdsEngine(engine),
					Address: awslib.StringValue(db.Endpoint.Address),
					Port:    int(awslib.Int64Value(db.Endpoint.Port)),
					Tags:    awsRDSTagsToMap(db.TagList),
				})
			}
			return true
		},
	)
	if err != nil {
		return nil, err
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
	var pageErr error
	err := c.docdb.DescribeDBClustersPagesWithContext(
		ctx,
		&docdb.DescribeDBClustersInput{},
		func(output *docdb.DescribeDBClustersOutput, _ bool) bool {
			for _, cluster := range output.DBClusters {
				if awslib.StringValue(cluster.Endpoint) == "" || awslib.Int64Value(cluster.Port) == 0 {
					continue
				}
				tags, err := c.docdbTags(ctx, awslib.StringValue(cluster.DBClusterArn))
				tags, err = tagsOrEmptyUnlessContextDone(ctx, tags, err)
				if err != nil {
					pageErr = err
					return false
				}
				instances = append(instances, MongoInstance{
					ID:      awslib.StringValue(cluster.DBClusterIdentifier),
					Name:    awslib.StringValue(cluster.DBClusterIdentifier),
					Region:  c.region,
					Address: awslib.StringValue(cluster.Endpoint),
					Port:    int(awslib.Int64Value(cluster.Port)),
					Tags:    tags,
				})
			}
			return true
		},
	)
	if err != nil {
		return nil, err
	}
	if pageErr != nil {
		return nil, pageErr
	}
	return instances, nil
}

func (c *SDKClient) DescribeEC2Instances(ctx context.Context) ([]EC2Instance, error) {
	var instances []EC2Instance
	err := c.ec2.DescribeInstancesPagesWithContext(
		ctx,
		&ec2.DescribeInstancesInput{},
		func(output *ec2.DescribeInstancesOutput, _ bool) bool {
			for _, reservation := range output.Reservations {
				for _, instance := range reservation.Instances {
					tags := ec2TagsToMap(instance.Tags)
					instances = append(instances, EC2Instance{
						ID:        awslib.StringValue(instance.InstanceId),
						Name:      firstNonEmpty(tags["Name"], awslib.StringValue(instance.InstanceId)),
						Region:    c.region,
						PrivateIP: awslib.StringValue(instance.PrivateIpAddress),
						PublicIP:  awslib.StringValue(instance.PublicIpAddress),
						Tags:      tags,
					})
				}
			}
			return true
		},
	)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

func (c *SDKClient) elasticacheTags(ctx context.Context, arn string) (map[string]string, error) {
	if arn == "" {
		return map[string]string{}, nil
	}
	output, err := c.elasticache.ListTagsForResourceWithContext(ctx, &elasticache.ListTagsForResourceInput{
		ResourceName: awslib.String(arn),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(output.TagList))
	for _, tag := range output.TagList {
		out[awslib.StringValue(tag.Key)] = awslib.StringValue(tag.Value)
	}
	return out, nil
}

func (c *SDKClient) docdbTags(ctx context.Context, arn string) (map[string]string, error) {
	if arn == "" {
		return map[string]string{}, nil
	}
	output, err := c.docdb.ListTagsForResourceWithContext(ctx, &docdb.ListTagsForResourceInput{
		ResourceName: awslib.String(arn),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(output.TagList))
	for _, tag := range output.TagList {
		out[awslib.StringValue(tag.Key)] = awslib.StringValue(tag.Value)
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

func awsRDSTagsToMap(tags []*rds.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := awslib.StringValue(tag.Key)
		if key == "" {
			continue
		}
		out[key] = awslib.StringValue(tag.Value)
	}
	return out
}

func ec2TagsToMap(tags []*ec2.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := awslib.StringValue(tag.Key)
		if key == "" {
			continue
		}
		out[key] = awslib.StringValue(tag.Value)
	}
	return out
}
