# cloud-sd

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Prometheus](https://img.shields.io/badge/Prometheus-HTTP%20SD-E6522C?logo=prometheus&logoColor=white)](docs/prometheus.md)
[![Status](https://img.shields.io/badge/status-v0.1.0-blue)](#project-status)

cloud-sd is a multi-cloud resource discovery service for Prometheus HTTP Service Discovery. It discovers managed databases, middleware, and compute resources from Alibaba Cloud and AWS, normalizes them into Prometheus `http_sd_configs` target groups, and lets Prometheus scrape them through exporters.

[中文文档](README.zh-CN.md) | [Prometheus integration](docs/prometheus.md) | [Example config](examples/config.yaml)

## Why cloud-sd?

Prometheus can discover many platforms natively, but managed cloud databases and middleware often need provider-specific APIs, tags, account metadata, and exporter-aware target routing. cloud-sd provides that normalization layer outside Prometheus.

Use cloud-sd when you want to:

- discover Redis, PostgreSQL, MySQL, MongoDB, and node targets from cloud APIs
- keep Prometheus scrape jobs stable while cloud resources change
- route cloud resources to multi-target exporters such as `redis_exporter`, `postgres_exporter`, `mysqld_exporter`, `mongodb_exporter`, and `node_exporter`
- filter discovered targets by tags, scopes, accounts, regions, and engines
- avoid maintaining static target files or periodic CMDB snapshots

## Features

- Prometheus HTTP SD endpoints for `/sd/redis`, `/sd/postgres`, `/sd/mysql`, `/sd/mongo`, and `/sd/node`
- Alibaba Cloud adapters for Redis/Tair, RDS MySQL, RDS PostgreSQL, MongoDB, and ECS
- AWS adapters for ElastiCache Redis/Valkey, RDS/Aurora MySQL, RDS/Aurora PostgreSQL, DocumentDB, and EC2
- tag/scope filtering with `cloud_sd_scope` and `cloud_sd_disable`
- dashboard-friendly labels: `vendor`, `account`, `account_id`, `region`, `group`, `name`, `iid`, `cservice`, `resource_type`, `engine`
- background refresh with in-memory snapshots, readiness checks, and source-level last-known-good behavior
- adapter/factory interfaces for future providers such as Huawei Cloud or CMDB-backed sources

## Project Status

cloud-sd `v0.1.0` is the first usable release. It includes Alibaba Cloud and AWS discovery adapters, Prometheus HTTP SD endpoints, exporter deployment examples, and multi-target scrape configuration.

This release intentionally keeps the runtime small:

- no UI
- no database dependency
- no file source
- no auth yet
- no persistent cache yet

The current focus is a clean adapter model, stable HTTP SD output, and practical Prometheus exporter integration.

## Supported Resources

| Endpoint | Engine | Alibaba Cloud | AWS |
|---|---|---|---|
| `/sd/redis` | `redis` | Redis / Tair | ElastiCache Redis / Valkey |
| `/sd/postgres` | `postgres` | RDS PostgreSQL | RDS PostgreSQL / Aurora PostgreSQL |
| `/sd/mysql` | `mysql` | RDS MySQL | RDS MySQL / Aurora MySQL |
| `/sd/mongo` | `mongo` | MongoDB | DocumentDB |
| `/sd/node` | `node` | ECS | EC2 |

`/sd/node` does not filter by instance status. Stopped or accidentally powered-off instances remain visible to Prometheus as unreachable targets. This is useful when power state changes should become monitoring signals.

## Architecture

![cloud-sd architecture](docs/assets/architecture.png)

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

Provider adapters query cloud APIs, normalize resources into a common model, and pass them through routing rules. Prometheus only reads HTTP SD snapshots; it never calls cloud APIs directly.

Every adapter implements:

```go
type ResourceSource interface {
    Name() string
    Provider() core.Provider
    ListResources(ctx context.Context) ([]core.Resource, error)
}
```

Provider factories build the enabled sources from configuration. The same path can later host Huawei Cloud, CMDB, MCP, or other inventory adapters without changing the Prometheus-facing API.

## Quick Start

1. Configure credentials. Environment variables are recommended:

```bash
export ALIYUN_PROD_ACCESS_KEY_ID="your-access-key-id"
export ALIYUN_PROD_ACCESS_KEY_SECRET="your-access-key-secret"
export AWS_ACCESS_KEY_ID="your-access-key-id"
export AWS_SECRET_ACCESS_KEY="your-secret-access-key"
```

2. Start cloud-sd:

```bash
go run ./cmd/cloud-sd -config examples/config.yaml
```

3. Check the service:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/sd/redis
```

## Configuration

cloud-sd uses YAML:

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

Notes:

- `collector.scopes` can be omitted or left empty to discover all non-disabled resources.
- `collector.engines` defaults to Redis only when omitted. If present, at least one engine must be enabled.
- `collector.refresh_interval` controls how often cloud-sd refreshes resources.
- `collector.refresh_timeout` is the deadline for one full refresh cycle.
- `collector.request_timeout` is passed to cloud SDK clients as the per-request timeout.
- `account_id` is resolved automatically through cloud STS APIs and cached in memory.
- Direct AK/SK values are supported for local testing, but environment variables or Kubernetes Secrets are recommended for production.
- AWS accepts both `access_key_secret` and `secret_access_key`; `session_token` is only needed for temporary STS credentials.

## Routing Rules

Resources are returned only when:

- the resource engine matches the requested `/sd/{engine}` endpoint
- `collector.scopes` is empty, or the configured scope tag value is listed in `collector.scopes`
- the configured disable tag is not `true`

Default tags:

```text
cloud_sd_scope=id1
cloud_sd_disable=false
```

Set this tag to disable discovery for a resource:

```text
cloud_sd_disable=true
```

Tag read permissions are required for scope and disable filtering to work correctly.

## HTTP API

| Endpoint | Description |
|---|---|
| `GET /sd/redis` | Redis-compatible targets |
| `GET /sd/postgres` | PostgreSQL-compatible targets |
| `GET /sd/mysql` | MySQL-compatible targets |
| `GET /sd/mongo` | MongoDB-compatible targets |
| `GET /sd/node` | Node exporter targets |
| `GET /healthz` | Liveness check |
| `GET /readyz` | Readiness and refresh status |
| `GET /metrics` | Reserved Prometheus metrics endpoint |

Example HTTP SD response:

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

## Prometheus Integration

For multi-target exporters, keep the discovered cloud resource address in `__param_target`, rewrite `__address__` to the exporter service, and copy `__param_target` to `instance`.

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
      - source_labels: [__address__]
        target_label: instance
      - target_label: __address__
        replacement: redis-exporter.monitoring.svc:9121
```

See [Prometheus Integration](docs/prometheus.md) for step-by-step guidance, [exporter scrape YAML snippets](docs/prometheus/exporters/), and [Kubernetes exporter install manifests](deploy/exporters/).

## Labels

cloud-sd emits labels that work well with Grafana dashboard variable chains such as:

```text
vendor -> account -> group -> name -> instance
```

| Label | Meaning |
|---|---|
| `vendor` | Cloud provider, such as `aliyun` or `aws` |
| `account` | Configured account name |
| `account_id` | Cloud account ID resolved by STS |
| `region` | Cloud region |
| `group` | Scope tag value |
| `name` | Cloud resource name |
| `iid` | Cloud resource ID |
| `cservice` | Service category, such as `redis` or `mysql` |
| `resource_type` | Normalized resource type |
| `engine` | Endpoint engine |

Node Exporter dashboards query metrics such as `node_uname_info`, `node_cpu_seconds_total`, and `node_memory_*`. Database exporters expose different metric names, so label compatibility helps with filtering and identity, but it does not make database metrics populate Node Exporter panels.

## Permissions

Use least-privilege cloud credentials.

Alibaba Cloud needs read permissions for:

- STS `GetCallerIdentity`
- Redis/Tair instance listing and tags
- RDS instance listing, instance details, and tags
- MongoDB instance listing, instance details, and tags
- ECS instance listing and tags

AWS needs read permissions for:

- STS `GetCallerIdentity`
- EC2 `DescribeInstances`
- ElastiCache `DescribeReplicationGroups` and `ListTagsForResource`
- RDS `DescribeDBInstances`
- DocumentDB `DescribeDBClusters` and `ListTagsForResource`

## Docker

```bash
docker build -t cloud-sd:local .
docker run --rm -p 8080:8080 \
  -e ALIYUN_PROD_ACCESS_KEY_ID \
  -e ALIYUN_PROD_ACCESS_KEY_SECRET \
  cloud-sd:local
```

For production, mount your own config file and inject credentials with your secret manager.

## Development

```bash
make test
make build
make run
```

The binary is written to `bin/cloud-sd`.

## Roadmap

- Huawei Cloud adapter
- Prometheus `client_golang` metrics
- HTTP endpoint auth
- optional read-only UI
- optional disk cache for the latest successful snapshot
- finer-grained per-account and per-region last-known-good cache
- identity resolution singleflight to reduce STS calls

## Contributing

Issues and pull requests are welcome. Keep provider adapters behind the `ResourceSource` / provider factory boundary, and include tests for routing, HTTP SD labels, and error handling.

## License

cloud-sd is licensed under the [Apache License 2.0](LICENSE).
