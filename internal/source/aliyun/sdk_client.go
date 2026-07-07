package aliyun

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/dds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r_kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
)

const (
	ecsPageSize               = 100
	redisPageSize             = 100
	rdsPageSize               = 100
	mongoPageSize             = 100
	aliyunTagResourceType     = "INSTANCE"
	aliyunTagResourceBatchMax = 50
	aliyunMongoDBEngine       = "MongoDB"
)

type SDKClient struct {
	region  string
	timeout time.Duration
	sts     *sts.Client
	redis   *r_kvstore.Client
	rds     *rds.Client
	dds     *dds.Client
	ecs     *ecs.Client
}

func NewSDKClient(account AccountConfig, region string, opts ClientOptions) (CloudClient, error) {
	credentials, err := account.ResolveCredentials()
	if err != nil {
		return nil, err
	}

	stsClient, err := sts.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create sts client: %w", err)
	}
	redisClient, err := r_kvstore.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create r-kvstore client: %w", err)
	}
	return &SDKClient{region: region, timeout: opts.RequestTimeout, sts: stsClient, redis: redisClient}, nil
}

func NewRDSSDKClient(account AccountConfig, region string, opts ClientOptions) (RDSClient, error) {
	credentials, err := account.ResolveCredentials()
	if err != nil {
		return nil, err
	}

	stsClient, err := sts.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create sts client: %w", err)
	}
	rdsClient, err := rds.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create rds client: %w", err)
	}
	return &SDKClient{region: region, timeout: opts.RequestTimeout, sts: stsClient, rds: rdsClient}, nil
}

func NewMongoSDKClient(account AccountConfig, region string, opts ClientOptions) (MongoClient, error) {
	credentials, err := account.ResolveCredentials()
	if err != nil {
		return nil, err
	}

	stsClient, err := sts.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create sts client: %w", err)
	}
	ddsClient, err := dds.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create dds client: %w", err)
	}
	return &SDKClient{region: region, timeout: opts.RequestTimeout, sts: stsClient, dds: ddsClient}, nil
}

func NewECSSDKClient(account AccountConfig, region string, opts ClientOptions) (ECSClient, error) {
	credentials, err := account.ResolveCredentials()
	if err != nil {
		return nil, err
	}

	stsClient, err := sts.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create sts client: %w", err)
	}
	ecsClient, err := ecs.NewClientWithAccessKey(region, credentials.AccessKeyID, credentials.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create ecs client: %w", err)
	}
	return &SDKClient{region: region, timeout: opts.RequestTimeout, sts: stsClient, ecs: ecsClient}, nil
}

func (c *SDKClient) GetCallerIdentity(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	request := sts.CreateGetCallerIdentityRequest()
	request.Scheme = "https"
	applyRequestTimeout(request, c.timeout)

	response, err := c.sts.GetCallerIdentity(request)
	if err != nil {
		return "", err
	}
	if response.AccountId == "" {
		return "", fmt.Errorf("sts GetCallerIdentity returned empty account id")
	}
	return response.AccountId, nil
}

func (c *SDKClient) DescribeRedisInstances(ctx context.Context) ([]RedisInstance, error) {
	var instances []RedisInstance
	for pageNumber := 1; ; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		request := r_kvstore.CreateDescribeInstancesRequest()
		request.Scheme = "https"
		request.PageNumber = requests.NewInteger(pageNumber)
		request.PageSize = requests.NewInteger(redisPageSize)
		applyRequestTimeout(request, c.timeout)

		response, err := c.redis.DescribeInstances(request)
		if err != nil {
			return nil, err
		}

		for _, item := range response.Instances.KVStoreInstance {
			instances = append(instances, RedisInstance{
				InstanceID:       item.InstanceId,
				InstanceName:     item.InstanceName,
				RegionID:         item.RegionId,
				ConnectionDomain: item.ConnectionDomain,
				Port:             int(item.Port),
				InstanceType:     item.InstanceType,
				Tags:             tagsToMap(item.Tags.Tag),
			})
		}

		if pageNumber*redisPageSize >= response.TotalCount || len(response.Instances.KVStoreInstance) == 0 {
			break
		}
	}
	return instances, nil
}

func (c *SDKClient) DescribeECSInstances(ctx context.Context) ([]ECSInstance, error) {
	var instances []ECSInstance
	for pageNumber := 1; ; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		request := newECSDescribeInstancesRequest(pageNumber, c.timeout)

		response, err := c.ecs.DescribeInstances(request)
		if err != nil {
			return nil, err
		}

		for _, item := range response.Instances.Instance {
			instances = append(instances, buildECSInstance(item, c.region))
		}

		if pageNumber*ecsPageSize >= response.TotalCount || len(response.Instances.Instance) == 0 {
			break
		}
	}
	return instances, nil
}

func newECSDescribeInstancesRequest(pageNumber int, timeout time.Duration) *ecs.DescribeInstancesRequest {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	request.PageNumber = requests.NewInteger(pageNumber)
	request.PageSize = requests.NewInteger(ecsPageSize)
	applyRequestTimeout(request, timeout)
	return request
}

func buildECSInstance(item ecs.Instance, fallbackRegion string) ECSInstance {
	return ECSInstance{
		InstanceID:   item.InstanceId,
		InstanceName: firstNonEmpty(item.InstanceName, item.HostName, item.Hostname, item.InstanceId),
		RegionID:     firstNonEmpty(item.RegionId, fallbackRegion),
		PrivateIP: firstNonEmpty(
			firstString(item.VpcAttributes.PrivateIpAddress.IpAddress),
			firstString(item.InnerIpAddress.IpAddress),
			item.IntranetIp,
		),
		PublicIP: firstNonEmpty(firstString(item.PublicIpAddress.IpAddress), item.InternetIp),
		EIP:      item.EipAddress.IpAddress,
		Tags:     ecsTagsToMap(item.Tags.Tag),
	}
}

func (c *SDKClient) DescribeRDSInstances(ctx context.Context, engine string) ([]RDSInstance, error) {
	var instances []RDSInstance
	for pageNumber := 1; ; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		request := rds.CreateDescribeDBInstancesRequest()
		request.Scheme = "https"
		request.Engine = engine
		request.PageNumber = requests.NewInteger(pageNumber)
		request.PageSize = requests.NewInteger(rdsPageSize)
		applyRequestTimeout(request, c.timeout)

		response, err := c.rds.DescribeDBInstances(request)
		if err != nil {
			return nil, err
		}

		pageItems := response.Items.DBInstance
		tagsByResourceID, err := c.listRDSTags(ctx, rdsInstanceIDs(pageItems))
		if err != nil {
			return nil, err
		}

		for _, item := range pageItems {
			attribute, err := c.describeRDSInstanceAttribute(ctx, item.DBInstanceId)
			if err != nil {
				return nil, err
			}
			instances = append(instances, buildRDSInstance(item, attribute, tagsByResourceID[item.DBInstanceId], c.region))
		}

		if pageNumber*rdsPageSize >= response.TotalRecordCount || len(pageItems) == 0 {
			break
		}
	}
	return instances, nil
}

func (c *SDKClient) describeRDSInstanceAttribute(ctx context.Context, instanceID string) (rds.DBInstanceAttribute, error) {
	if err := ctx.Err(); err != nil {
		return rds.DBInstanceAttribute{}, err
	}
	request := rds.CreateDescribeDBInstanceAttributeRequest()
	request.Scheme = "https"
	request.DBInstanceId = instanceID
	applyRequestTimeout(request, c.timeout)

	response, err := c.rds.DescribeDBInstanceAttribute(request)
	if err != nil {
		return rds.DBInstanceAttribute{}, err
	}
	if len(response.Items.DBInstanceAttribute) == 0 {
		return rds.DBInstanceAttribute{}, nil
	}
	return response.Items.DBInstanceAttribute[0], nil
}

func buildRDSInstance(item rds.DBInstance, attribute rds.DBInstanceAttribute, tags map[string]string, fallbackRegion string) RDSInstance {
	instanceID := firstNonEmpty(attribute.DBInstanceId, item.DBInstanceId)
	return RDSInstance{
		InstanceID:       instanceID,
		InstanceName:     firstNonEmpty(attribute.DBInstanceDescription, item.DBInstanceDescription, item.DBInstanceName),
		RegionID:         firstNonEmpty(attribute.RegionId, item.RegionId, fallbackRegion),
		ConnectionString: firstNonEmpty(attribute.ConnectionString, item.ConnectionString),
		Port:             parsePort(attribute.Port),
		Engine:           firstNonEmpty(attribute.Engine, item.Engine),
		Tags:             copyMap(tags),
	}
}

func (c *SDKClient) listRDSTags(ctx context.Context, resourceIDs []string) (map[string]map[string]string, error) {
	tagsByResourceID := map[string]map[string]string{}
	for _, batch := range chunkStrings(resourceIDs, aliyunTagResourceBatchMax) {
		nextToken := ""
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			request := newRDSListTagResourcesRequest(batch, nextToken, c.timeout)

			response, err := c.rds.ListTagResources(request)
			if err != nil {
				return nil, err
			}
			for _, tag := range response.TagResources.TagResource {
				if tag.ResourceId == "" || tag.TagKey == "" {
					continue
				}
				if tagsByResourceID[tag.ResourceId] == nil {
					tagsByResourceID[tag.ResourceId] = map[string]string{}
				}
				tagsByResourceID[tag.ResourceId][tag.TagKey] = tag.TagValue
			}
			if response.NextToken == "" {
				break
			}
			nextToken = response.NextToken
		}
	}
	return tagsByResourceID, nil
}

func (c *SDKClient) DescribeMongoInstances(ctx context.Context) ([]MongoInstance, error) {
	var instances []MongoInstance
	for pageNumber := 1; ; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		request := dds.CreateDescribeDBInstancesRequest()
		request.Scheme = "https"
		request.Engine = aliyunMongoDBEngine
		request.PageNumber = requests.NewInteger(pageNumber)
		request.PageSize = requests.NewInteger(mongoPageSize)
		applyRequestTimeout(request, c.timeout)

		response, err := c.dds.DescribeDBInstances(request)
		if err != nil {
			return nil, err
		}

		pageItems := response.DBInstances.DBInstance
		tagsByResourceID, err := c.listMongoTags(ctx, mongoInstanceIDs(pageItems))
		if err != nil {
			return nil, err
		}

		for _, item := range pageItems {
			attribute, err := c.describeMongoInstanceAttribute(ctx, item.DBInstanceId)
			if err != nil {
				return nil, err
			}
			instances = append(instances, buildMongoInstance(item, attribute, tagsByResourceID[item.DBInstanceId], c.region))
		}

		if pageNumber*mongoPageSize >= response.TotalCount || len(pageItems) == 0 {
			break
		}
	}
	return instances, nil
}

func (c *SDKClient) describeMongoInstanceAttribute(ctx context.Context, instanceID string) (dds.DBInstance, error) {
	if err := ctx.Err(); err != nil {
		return dds.DBInstance{}, err
	}
	request := dds.CreateDescribeDBInstanceAttributeRequest()
	request.Scheme = "https"
	request.Engine = aliyunMongoDBEngine
	request.DBInstanceId = instanceID
	applyRequestTimeout(request, c.timeout)

	response, err := c.dds.DescribeDBInstanceAttribute(request)
	if err != nil {
		return dds.DBInstance{}, err
	}
	if len(response.DBInstances.DBInstance) == 0 {
		return dds.DBInstance{}, nil
	}
	return response.DBInstances.DBInstance[0], nil
}

func buildMongoInstance(item dds.DBInstance, attribute dds.DBInstance, tags map[string]string, fallbackRegion string) MongoInstance {
	connectionString, port := mongoEndpoint(attribute)
	if connectionString == "" || port == 0 {
		connectionString, port = mongoEndpoint(item)
	}
	instanceID := firstNonEmpty(attribute.DBInstanceId, item.DBInstanceId)
	return MongoInstance{
		InstanceID:       instanceID,
		InstanceName:     firstNonEmpty(attribute.DBInstanceDescription, item.DBInstanceDescription),
		RegionID:         firstNonEmpty(attribute.RegionId, item.RegionId, fallbackRegion),
		ConnectionString: connectionString,
		Port:             port,
		Tags:             copyMap(tags),
	}
}

func mongoEndpoint(instance dds.DBInstance) (string, int) {
	for _, address := range instance.NetworkAddresses.NetworkAddress {
		port := parsePort(address.Port)
		if address.NetworkAddress != "" && port != 0 {
			return address.NetworkAddress, port
		}
	}
	for _, replicaSet := range instance.ReplicaSets.ReplicaSet {
		port := parsePort(replicaSet.ConnectionPort)
		if replicaSet.ConnectionDomain != "" && port != 0 {
			return replicaSet.ConnectionDomain, port
		}
	}
	for _, mongos := range instance.MongosList.MongosAttribute {
		if mongos.ConnectSting != "" && mongos.Port != 0 {
			return mongos.ConnectSting, mongos.Port
		}
	}
	for _, shard := range instance.ShardList.ShardAttribute {
		if shard.ConnectString != "" && shard.Port != 0 {
			return shard.ConnectString, shard.Port
		}
	}
	return "", 0
}

func (c *SDKClient) listMongoTags(ctx context.Context, resourceIDs []string) (map[string]map[string]string, error) {
	tagsByResourceID := map[string]map[string]string{}
	for _, batch := range chunkStrings(resourceIDs, aliyunTagResourceBatchMax) {
		nextToken := ""
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			request := newMongoListTagResourcesRequest(batch, nextToken, c.timeout)

			response, err := c.dds.ListTagResources(request)
			if err != nil {
				return nil, err
			}
			for _, tag := range response.TagResources.TagResource {
				if tag.ResourceId == "" || tag.TagKey == "" {
					continue
				}
				if tagsByResourceID[tag.ResourceId] == nil {
					tagsByResourceID[tag.ResourceId] = map[string]string{}
				}
				tagsByResourceID[tag.ResourceId][tag.TagKey] = tag.TagValue
			}
			if response.NextToken == "" {
				break
			}
			nextToken = response.NextToken
		}
	}
	return tagsByResourceID, nil
}

func rdsInstanceIDs(instances []rds.DBInstance) []string {
	ids := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance.DBInstanceId != "" {
			ids = append(ids, instance.DBInstanceId)
		}
	}
	return ids
}

func mongoInstanceIDs(instances []dds.DBInstance) []string {
	ids := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance.DBInstanceId != "" {
			ids = append(ids, instance.DBInstanceId)
		}
	}
	return ids
}

func newRDSListTagResourcesRequest(resourceIDs []string, nextToken string, timeout time.Duration) *rds.ListTagResourcesRequest {
	request := rds.CreateListTagResourcesRequest()
	request.Scheme = "https"
	request.ResourceType = aliyunTagResourceType
	request.NextToken = nextToken
	if len(resourceIDs) > 0 {
		ids := append([]string(nil), resourceIDs...)
		request.ResourceId = &ids
	}
	applyRequestTimeout(request, timeout)
	return request
}

func newMongoListTagResourcesRequest(resourceIDs []string, nextToken string, timeout time.Duration) *dds.ListTagResourcesRequest {
	request := dds.CreateListTagResourcesRequest()
	request.Scheme = "https"
	request.ResourceType = aliyunTagResourceType
	request.NextToken = nextToken
	if len(resourceIDs) > 0 {
		ids := append([]string(nil), resourceIDs...)
		request.ResourceId = &ids
	}
	applyRequestTimeout(request, timeout)
	return request
}

func chunkStrings(values []string, size int) [][]string {
	if len(values) == 0 {
		return nil
	}
	if size <= 0 || len(values) <= size {
		return [][]string{append([]string(nil), values...)}
	}

	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, append([]string(nil), values[start:end]...))
	}
	return chunks
}

type timeoutRequest interface {
	SetReadTimeout(readTimeout time.Duration)
	SetConnectTimeout(connectTimeout time.Duration)
}

func applyRequestTimeout(request timeoutRequest, timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	request.SetReadTimeout(timeout)
	request.SetConnectTimeout(timeout)
}

func tagsToMap(tags []r_kvstore.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[tag.Key] = tag.Value
	}
	return out
}

func ecsTagsToMap(tags []ecs.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := firstNonEmpty(tag.Key, tag.TagKey)
		if key == "" {
			continue
		}
		out[key] = firstNonEmpty(tag.Value, tag.TagValue)
	}
	return out
}

func firstString(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parsePort(value string) int {
	if value == "" {
		return 0
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return port
}
