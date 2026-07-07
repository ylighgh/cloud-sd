# cloud-sd

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Prometheus](https://img.shields.io/badge/Prometheus-HTTP%20SD-E6522C?logo=prometheus&logoColor=white)](docs/prometheus.zh-CN.md)
[![Status](https://img.shields.io/badge/status-MVP-blue)](#项目状态)

cloud-sd 是一个面向 Prometheus HTTP Service Discovery 的多云资源发现服务。它从 Alibaba Cloud 和 AWS 发现托管数据库、中间件和计算资源，归一化为 Prometheus `http_sd_configs` 兼容的 target groups，让 Prometheus 可以配合 exporter 进行采集。

[English](README.md) | [Prometheus 集成](docs/prometheus.zh-CN.md) | [示例配置](examples/config.yaml)

## 为什么需要 cloud-sd？

Prometheus 原生支持很多 service discovery，但云数据库和中间件通常需要云厂商 API、tag、账号元数据，以及面向 exporter 的 target 路由。cloud-sd 把这层归一化和路由逻辑从 Prometheus 中抽出来。

适合这些场景：

- 从云 API 发现 Redis、PostgreSQL、MySQL、MongoDB 和 Node targets
- 云资源动态变化时，保持 Prometheus scrape jobs 稳定
- 把云资源路由到 `redis_exporter`、`postgres_exporter`、`mysqld_exporter`、`mongodb_exporter`、`node_exporter` 等 multi-target exporter
- 按 tag、scope、account、region、engine 过滤发现结果
- 替代静态 target 文件或周期性 CMDB 快照

## 功能特性

- Prometheus HTTP SD endpoints：`/sd/redis`、`/sd/postgres`、`/sd/mysql`、`/sd/mongo`、`/sd/node`
- Alibaba Cloud adapters：Redis/Tair、RDS MySQL、RDS PostgreSQL、MongoDB、ECS
- AWS adapters：ElastiCache Redis/Valkey、RDS/Aurora MySQL、RDS/Aurora PostgreSQL、DocumentDB、EC2
- 支持 `cloud_sd_scope` 和 `cloud_sd_disable` 的 tag/scope 过滤
- 面向看板的 labels：`vendor`、`account`、`account_id`、`region`、`group`、`name`、`iid`、`cservice`、`resource_type`、`engine`
- 后台刷新、内存快照、readyz 状态，以及 source 级 last-known-good 行为
- 为 Huawei Cloud、CMDB、MCP 或其他资源源预留 adapter/factory 扩展点

## 项目状态

cloud-sd 当前处于 MVP 阶段。第一版刻意保持运行时轻量：

- 不做 UI
- 不依赖数据库
- 不做 file source
- 暂无认证
- 暂无持久化缓存

当前重点是清晰的 adapter 模型、稳定的 HTTP SD 输出，以及实用的 Prometheus exporter 集成。

## 支持的资源

| Endpoint | Engine | Alibaba Cloud | AWS |
|---|---|---|---|
| `/sd/redis` | `redis` | Redis / Tair | ElastiCache Redis / Valkey |
| `/sd/postgres` | `postgres` | RDS PostgreSQL | RDS PostgreSQL / Aurora PostgreSQL |
| `/sd/mysql` | `mysql` | RDS MySQL | RDS MySQL / Aurora MySQL |
| `/sd/mongo` | `mongo` | MongoDB | DocumentDB |
| `/sd/node` | `node` | ECS | EC2 |

`/sd/node` 不按实例状态过滤。停止或误关机的实例会继续暴露给 Prometheus，并表现为不可达 target，这样电源状态变化也能成为监控信号。

## 架构

![cloud-sd 架构图](docs/assets/architecture.zh-CN.png)

```text
Cloud accounts and regions
        |
        v
ResourceSource adapters
        |
        v
MultiSource aggregator
        |
        v
In-memory snapshot store
        |
        v
Prometheus HTTP SD endpoints
```

Provider adapter 调用云 API，把资源归一化成统一模型，再经过 routing rules 过滤。Prometheus 只读取 HTTP SD 快照，不会直接调用云 API。

每个 adapter 实现同一个接口：

```go
type ResourceSource interface {
    Name() string
    Provider() core.Provider
    ListResources(ctx context.Context) ([]core.Resource, error)
}
```

Provider factory 根据配置中启用的 engines 构建对应 sources。后续 Huawei Cloud、CMDB、MCP 或其他 inventory adapters 可以接入同一条路径，不需要改变 Prometheus-facing API。

## 快速启动

1. 配置凭证。推荐使用环境变量：

```bash
export ALIYUN_PROD_ACCESS_KEY_ID="your-access-key-id"
export ALIYUN_PROD_ACCESS_KEY_SECRET="your-access-key-secret"
export AWS_ACCESS_KEY_ID="your-access-key-id"
export AWS_SECRET_ACCESS_KEY="your-secret-access-key"
```

2. 启动 cloud-sd：

```bash
go run ./cmd/cloud-sd -config examples/config.yaml
```

3. 验证接口：

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/sd/redis
```

## 配置

cloud-sd 使用 YAML：

```yaml
server:
  listen: ":8080"

collector:
  scopes:
    - id1
    - game-id1
  engines:
    redis: true
    mysql: true
    postgres: true
    mongo: true
    node: true
  refresh_interval: 5m
  refresh_timeout: 1m
  request_timeout: 20s

routing:
  scope_tag: cloud_sd_scope
  disable_tag: cloud_sd_disable

aliyun:
  enabled: true
  accounts:
    - name: prod
      regions:
        - cn-hangzhou
        - ap-southeast-1
      access_key_id_env: ALIYUN_PROD_ACCESS_KEY_ID
      access_key_secret_env: ALIYUN_PROD_ACCESS_KEY_SECRET

aws:
  enabled: true
  accounts:
    - name: aws-prod
      regions:
        - ap-southeast-1
      access_key_id_env: AWS_ACCESS_KEY_ID
      secret_access_key_env: AWS_SECRET_ACCESS_KEY
```

说明：

- `collector.scopes` 可以省略或留空，表示发现所有未禁用资源。
- `collector.engines` 省略时默认只启用 Redis；如果显式配置，至少要启用一个 engine。
- `collector.refresh_interval` 控制资源刷新频率。
- `collector.refresh_timeout` 是一整轮刷新 deadline。
- `collector.request_timeout` 会传给云 SDK client，作为单次请求超时。
- `account_id` 会通过云厂商 STS API 自动解析，并缓存在内存中。
- 本地测试可以直接写 AK/SK；生产环境推荐环境变量或 Kubernetes Secrets。
- AWS 同时兼容 `access_key_secret` 和 `secret_access_key`；`session_token` 只在 STS 临时凭证场景需要。

## 路由规则

资源只有满足以下条件才会返回：

- resource engine 与请求的 `/sd/{engine}` endpoint 匹配
- `collector.scopes` 为空，或配置的 scope tag 值在 `collector.scopes` 中
- 配置的 disable tag 不是 `true`

默认 tag：

```text
cloud_sd_scope=id1
cloud_sd_disable=false
```

禁用资源发现：

```text
cloud_sd_disable=true
```

scope 和 disable 过滤依赖云资源 tag 读取权限。

## HTTP API

| Endpoint | 说明 |
|---|---|
| `GET /sd/redis` | Redis-compatible targets |
| `GET /sd/postgres` | PostgreSQL-compatible targets |
| `GET /sd/mysql` | MySQL-compatible targets |
| `GET /sd/mongo` | MongoDB-compatible targets |
| `GET /sd/node` | Node exporter targets |
| `GET /healthz` | 存活检查 |
| `GET /readyz` | 就绪状态和刷新状态 |
| `GET /metrics` | 预留的 Prometheus metrics endpoint |

HTTP SD 返回示例：

```json
[
  {
    "targets": ["redis.example.com:6379"],
    "labels": {
      "vendor": "aliyun",
      "account": "prod",
      "account_id": "123456789",
      "region": "cn-hangzhou",
      "group": "id1",
      "name": "prod-redis-cache",
      "iid": "r-bp123",
      "cservice": "redis",
      "resource_type": "redis_instance",
      "engine": "redis"
    }
  }
]
```

## Prometheus 集成

对于 multi-target exporter，把 cloud-sd 发现到的云资源地址保留到 `__param_target`，再把 `__address__` 改写成 exporter 服务地址，并把 `__param_target` 写回 `instance`。

```yaml
scrape_configs:
  - job_name: cloud-redis
    metrics_path: /scrape
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/redis
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - target_label: __address__
        replacement: redis-exporter.monitoring.svc:9121
      - source_labels: [__param_target]
        target_label: instance
```

Redis、PostgreSQL、MySQL、MongoDB 和 Node exporter 的完整示例见 [Prometheus 集成](docs/prometheus.zh-CN.md)。

## Labels

cloud-sd 输出适合 Grafana dashboard 变量链的 labels：

```text
vendor -> account -> group -> name -> instance
```

| Label | 含义 |
|---|---|
| `vendor` | 云厂商，例如 `aliyun` 或 `aws` |
| `account` | 配置文件中的账号名 |
| `account_id` | STS 解析出的云账号 ID |
| `region` | 云 region |
| `group` | scope tag 值 |
| `name` | 云资源名称 |
| `iid` | 云资源 ID |
| `cservice` | 服务类别，例如 `redis` 或 `mysql` |
| `resource_type` | 归一化资源类型 |
| `engine` | endpoint engine |

Node Exporter 看板查询的是 `node_uname_info`、`node_cpu_seconds_total`、`node_memory_*` 等指标。数据库 exporter 的指标名不同，所以 label 兼容解决的是筛选和身份维度，不会让数据库指标直接填充 Node Exporter 面板。

## 权限

建议使用最小权限云凭证。

Alibaba Cloud 需要读取：

- STS `GetCallerIdentity`
- Redis/Tair 实例列表和 tags
- RDS 实例列表、实例详情和 tags
- MongoDB 实例列表、实例详情和 tags
- ECS 实例列表和 tags

AWS 需要读取：

- STS `GetCallerIdentity`
- EC2 `DescribeInstances`
- ElastiCache `DescribeReplicationGroups` 和 `ListTagsForResource`
- RDS `DescribeDBInstances`
- DocumentDB `DescribeDBClusters` 和 `ListTagsForResource`

## Docker

```bash
docker build -t cloud-sd:local .
docker run --rm -p 8080:8080 \
  -e ALIYUN_PROD_ACCESS_KEY_ID \
  -e ALIYUN_PROD_ACCESS_KEY_SECRET \
  cloud-sd:local
```

生产环境建议挂载自己的配置文件，并通过密钥管理系统注入凭证。

## 开发

```bash
make test
make build
make run
```

构建产物会写入 `bin/cloud-sd`。

## Roadmap

- Huawei Cloud adapter
- Prometheus `client_golang` metrics
- HTTP endpoint auth
- optional read-only UI
- optional disk cache for the latest successful snapshot
- 更细粒度的 account / region last-known-good cache
- identity resolution singleflight，减少 STS 调用

## 贡献

欢迎提交 issue 和 pull request。Provider adapter 建议保持在 `ResourceSource` / provider factory 边界内，并补充 routing、HTTP SD labels、错误处理相关测试。

## License

当前还没有声明 license。正式作为开源项目发布前，建议增加 `LICENSE` 文件。
