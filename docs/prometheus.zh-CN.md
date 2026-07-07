# Prometheus 集成

[English](prometheus.md)

本文说明如何把 cloud-sd 接入 Prometheus HTTP service discovery，并分别配置 Redis、MySQL、PostgreSQL、MongoDB 和 Node Exporter 的多实例采集链路。

## 整体采集模型

cloud-sd 只负责发现云资源地址，并输出 Prometheus HTTP SD target groups。Prometheus 负责拉取 cloud-sd 的 `/sd/*` 接口，再通过 relabel 把发现到的地址传给 exporter。

```text
cloud-sd /sd/{engine}
        |
        v
Prometheus http_sd_configs
        |
        v
relabel: discovered target -> exporter query parameter
        |
        v
Exporter
        |
        v
Cloud resource metrics
```

推荐 job 命名不要绑定具体云厂商：

```text
cloud-redis
cloud-mysql
cloud-postgres
cloud-mongo
cloud-node
```

这样后续同一个 job 可以同时承载 Alibaba Cloud 和 AWS 资源。

## 前置准备

1. 启动 cloud-sd，并确认 `/readyz` 已经 ready：

```bash
curl http://cloud-sd:8080/healthz
curl http://cloud-sd:8080/readyz
```

2. 确认各 engine endpoint 可以返回 target groups：

```bash
curl http://cloud-sd:8080/sd/redis
curl http://cloud-sd:8080/sd/mysql
curl http://cloud-sd:8080/sd/postgres
curl http://cloud-sd:8080/sd/mongo
curl http://cloud-sd:8080/sd/node
```

3. 确认云资源 tag 已配置好：

```text
cloud_sd_scope=id1
cloud_sd_disable=false
```

`collector.scopes` 为空时，会发现所有未禁用资源。设置 `cloud_sd_disable=true` 后，该资源不会出现在 `/sd/*` 结果中。

## 通用 relabel 模式

数据库和中间件 exporter 通常使用 multi-target 模式。Prometheus 发现到的 `__address__` 是云资源地址，例如 `redis.example.com:6379`。relabel 后，Prometheus 实际请求 exporter，并把云资源地址作为 query 参数传给 exporter。

通用模式：

```yaml
relabel_configs:
  - source_labels: [__address__]
    regex: (.+)
    target_label: __param_target
    replacement: $1
  - source_labels: [__param_target]
    target_label: instance
  - target_label: __address__
    replacement: exporter-service.monitoring.svc:PORT
```

如果 exporter 要求 URI 格式，可以在 `replacement` 中加协议前缀：

```yaml
relabel_configs:
  - source_labels: [__address__]
    regex: (.+)
    target_label: __param_target
    replacement: redis://$1
```

`instance` 建议设置为真实云资源地址，而不是 exporter 服务地址，这样 Grafana 和告警里看到的是被采集资源。

## Redis Exporter 多实例采集

适用场景：

- cloud-sd 的 `/sd/redis` 返回 Redis/Tair 或 ElastiCache 地址
- 一个 redis_exporter 通过 `target` 参数探测多个 Redis 实例
- Redis 实例使用相同认证方式，或 exporter 自己能处理认证配置

### 步骤 1：部署 redis_exporter

示例服务地址：

```text
redis-exporter.monitoring.svc:9121
```

如果 Redis 需要密码，建议通过 exporter 的环境变量、配置文件或 Secret 注入，不要把密码放到 cloud-sd label 里。

### 步骤 2：配置 Prometheus job

```yaml
scrape_configs:
  - job_name: cloud-redis
    metrics_path: /scrape
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/redis
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        regex: (.+)
        target_label: __param_target
        replacement: redis://$1
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: redis-exporter.monitoring.svc:9121
```

最终请求形式类似：

```text
http://redis-exporter.monitoring.svc:9121/scrape?target=redis://redis.example.com:6379
```

### 步骤 3：验证

```promql
up{job="cloud-redis"}
redis_up{job="cloud-redis"}
```

如果 `up=1` 但 `redis_up=0`，通常说明 exporter 可访问，但 Redis 认证、网络或 Redis 实例本身不可用。

## MySQL Exporter 多实例采集

适用场景：

- cloud-sd 的 `/sd/mysql` 返回 RDS MySQL 或 Aurora MySQL 地址
- mysqld_exporter 使用 `/probe` multi-target 模式
- MySQL 凭证由 exporter 的配置文件或 Secret 管理

### 步骤 1：部署 mysqld_exporter

示例服务地址：

```text
mysqld-exporter.monitoring.svc:9104
```

建议在 exporter 配置中准备一个云数据库采集账号，例如 `client.cloud`。Prometheus 只传递 target，不传递账号密码。

示例 exporter auth module 名称：

```text
client.cloud
```

### 步骤 2：配置 Prometheus job

```yaml
scrape_configs:
  - job_name: cloud-mysql
    metrics_path: /probe
    params:
      auth_module: [client.cloud]
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/mysql
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: mysqld-exporter.monitoring.svc:9104
```

最终请求形式类似：

```text
http://mysqld-exporter.monitoring.svc:9104/probe?auth_module=client.cloud&target=mysql.example.com:3306
```

### 步骤 3：验证

```promql
up{job="cloud-mysql"}
mysql_up{job="cloud-mysql"}
```

如果 exporter 版本不支持 `/probe` 或 `auth_module`，需要升级 exporter，或按账号/凭证组拆分多个 exporter 服务。

## PostgreSQL Exporter 多实例采集

适用场景：

- cloud-sd 的 `/sd/postgres` 返回 RDS PostgreSQL 或 Aurora PostgreSQL 地址
- postgres_exporter 使用 `/probe` multi-target 模式
- PostgreSQL 凭证由 exporter 的 auth module 或 Secret 管理

### 步骤 1：部署 postgres_exporter

示例服务地址：

```text
postgres-exporter.monitoring.svc:9187
```

准备一个 exporter auth module，例如：

```text
cloud
```

### 步骤 2：配置 Prometheus job

很多 postgres_exporter 的 multi-target 模式期望 `target` 是 PostgreSQL URI。cloud-sd 返回的是 `host:port`，所以 relabel 时可以拼成 URI：

```yaml
scrape_configs:
  - job_name: cloud-postgres
    metrics_path: /probe
    params:
      auth_module: [cloud]
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/postgres
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        regex: (.+)
        target_label: __param_target
        replacement: postgresql://$1/postgres?sslmode=disable
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: postgres-exporter.monitoring.svc:9187
```

最终请求形式类似：

```text
http://postgres-exporter.monitoring.svc:9187/probe?auth_module=cloud&target=postgresql://pg.example.com:5432/postgres?sslmode=disable
```

### 步骤 3：验证

```promql
up{job="cloud-postgres"}
pg_up{job="cloud-postgres"}
```

如果你的 exporter 期望 `target=host:port` 而不是 URI，去掉 `postgresql://` 前缀和路径即可。

## MongoDB Exporter 多实例采集

适用场景：

- cloud-sd 的 `/sd/mongo` 返回 MongoDB 或 DocumentDB 地址
- mongodb_exporter 支持通过 query 参数接收目标地址
- MongoDB 认证由 exporter 配置或 Secret 管理

### 步骤 1：部署 mongodb_exporter

示例服务地址：

```text
mongodb-exporter.monitoring.svc:9216
```

不同 mongodb_exporter 实现的 multi-target 参数和路径可能不同。下面以支持 `/scrape?target=` 的 exporter 为例。

### 步骤 2：配置 Prometheus job

```yaml
scrape_configs:
  - job_name: cloud-mongo
    metrics_path: /scrape
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/mongo
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        regex: (.+)
        target_label: __param_target
        replacement: mongodb://$1
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: mongodb-exporter.monitoring.svc:9216
```

最终请求形式类似：

```text
http://mongodb-exporter.monitoring.svc:9216/scrape?target=mongodb://mongo.example.com:27017
```

### 步骤 3：验证

```promql
up{job="cloud-mongo"}
mongodb_up{job="cloud-mongo"}
```

如果采集的是 AWS DocumentDB，通常还需要按你的 exporter 和 TLS 策略处理 CA、TLS 和兼容参数。

## Node Exporter 多实例采集

`/sd/node` 和数据库、中间件 exporter 不一样。cloud-sd 返回的是 ECS/EC2 实例地址加 Node Exporter 默认端口 `9100`，Prometheus 可以直接抓取这些主机上的 Node Exporter。

### 步骤 1：在云主机上运行 node_exporter

每台 ECS/EC2 都需要运行 node_exporter，并确保 Prometheus 能访问：

```text
<ecs-or-ec2-private-ip>:9100
```

常见部署方式：

- systemd service
- Ansible / Terraform / cloud-init
- Kubernetes 节点上用 DaemonSet，但这种情况下更常见的是 Kubernetes SD

### 步骤 2：配置 Prometheus job

```yaml
scrape_configs:
  - job_name: cloud-node
    metrics_path: /metrics
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/node
        refresh_interval: 60s
```

不需要把 `__address__` 改写到 exporter 服务，因为目标主机本身就是 node_exporter endpoint。

如果你希望 `instance` 只保留 IP 或保留 cloud-sd 输出的资源名，可以额外加 relabel；默认情况下 Prometheus 会把 `instance` 设为 `host:port`。

### 步骤 3：验证

```promql
up{job="cloud-node"}
node_uname_info{job="cloud-node"}
node_cpu_seconds_total{job="cloud-node"}
```

cloud-sd 不过滤 ECS/EC2 的运行状态。停止或误关机实例仍会出现在 `/sd/node` 中，并在 Prometheus 中表现为 `up=0`。

## 完整 scrape_configs 示例

下面示例假设 exporter 服务都部署在 `monitoring` 命名空间或同等网络域名下：

```yaml
scrape_configs:
  - job_name: cloud-redis
    metrics_path: /scrape
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/redis
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        regex: (.+)
        target_label: __param_target
        replacement: redis://$1
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: redis-exporter.monitoring.svc:9121

  - job_name: cloud-mysql
    metrics_path: /probe
    params:
      auth_module: [client.cloud]
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/mysql
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: mysqld-exporter.monitoring.svc:9104

  - job_name: cloud-postgres
    metrics_path: /probe
    params:
      auth_module: [cloud]
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/postgres
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        regex: (.+)
        target_label: __param_target
        replacement: postgresql://$1/postgres?sslmode=disable
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: postgres-exporter.monitoring.svc:9187

  - job_name: cloud-mongo
    metrics_path: /scrape
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/mongo
        refresh_interval: 60s
    relabel_configs:
      - source_labels: [__address__]
        regex: (.+)
        target_label: __param_target
        replacement: mongodb://$1
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: mongodb-exporter.monitoring.svc:9216

  - job_name: cloud-node
    metrics_path: /metrics
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/node
        refresh_interval: 60s
```

## Labels

cloud-sd 会返回类似下面的 labels：

```json
{
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
```

Prometheus 会把这些 labels 附加到发现出的 target 上。你可以在 relabel、告警规则、看板和 recording rules 中使用这些 labels。

## Grafana Dashboard Label 兼容

HTTP SD label 集合兼容 TenSunS / Node Exporter 风格看板常见变量链：

```text
vendor -> account -> group -> name -> instance
```

字段含义：

| Label | 含义 |
|---|---|
| `vendor` | 云厂商，例如 `aliyun` 或 `aws` |
| `account` | 配置里的云账号名称，例如 `accounts[].name` |
| `account_id` | STS 自动解析出的真实云账号 ID |
| `region` | 云地域 |
| `group` | scope tag 的值，默认来自 `cloud_sd_scope` |
| `name` | 云资源展示名 |
| `iid` | 云资源实例 ID |
| `cservice` | 服务类别，MVP 中和 `engine` 保持一致 |
| `resource_type` | cloud-sd 资源类型 |
| `engine` | `redis`、`mysql`、`postgres`、`mongo` 或 `node` |

cloud-sd 不再输出旧 labels：`provider`、`account_name`、`region_id`、`resource_id`、`resource_name`、`scope`。

## 验证和排障

### 1. 在 Prometheus 查看服务发现结果

打开 Prometheus UI：

```text
Status -> Service Discovery
Status -> Targets
```

检查 `cloud-redis`、`cloud-mysql`、`cloud-postgres`、`cloud-mongo`、`cloud-node` 是否出现 target。

### 2. 检查 up 指标

```promql
up{job=~"cloud-.*"}
```

### 3. 检查 labels 是否正确

```promql
count by (job, vendor, account, region, group, engine) (up{job=~"cloud-.*"})
```

### 4. 常见问题

| 现象 | 常见原因 | 处理方式 |
|---|---|---|
| `/sd/*` 返回 503 | cloud-sd 还没 ready 或首次刷新失败 | 查看 `/readyz` 和 cloud-sd 日志 |
| Prometheus 没有 target | `http_sd_configs.url` 不可达 | 从 Prometheus 容器里 curl cloud-sd |
| target 存在但 `up=0` | exporter 无法探测目标资源 | 查看 exporter 日志、网络 ACL、安全组、数据库白名单 |
| `instance` 变成 exporter 地址 | 缺少 `__param_target -> instance` relabel | 补充 relabel 规则 |
| scope 过滤不生效 | 云资源 tag 缺失或 tag API 权限不足 | 检查 `cloud_sd_scope` 和 tag 读取权限 |
| 禁用资源仍出现 | `cloud_sd_disable=true` 未配置或 tag 读取失败 | 检查资源 tag 和 cloud-sd refresh 日志 |

### 5. 指标名提醒

Node Exporter 看板要求 `node_uname_info`、`node_cpu_seconds_total`、`node_memory_*` 等 Node Exporter 指标。数据库 exporter 的指标名不同，所以 label 兼容解决的是身份和筛选维度，不会让数据库指标直接填充 Node Exporter 面板。
