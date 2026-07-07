# Prometheus Integration

[中文文档](prometheus.zh-CN.md)

This document explains how to integrate cloud-sd with Prometheus HTTP service discovery and configure multi-instance scraping for Redis, MySQL, PostgreSQL, MongoDB, and Node Exporter targets.

## Scraping Model

cloud-sd discovers cloud resource addresses and exposes Prometheus HTTP SD target groups. Prometheus reads the `/sd/*` endpoints, then relabels discovered addresses into exporter query parameters.

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

Use cloud-neutral job names:

```text
cloud-redis
cloud-mysql
cloud-postgres
cloud-mongo
cloud-node
```

This allows one job to hold targets from Alibaba Cloud and AWS.

## Prerequisites

1. Start cloud-sd and check readiness:

```bash
curl http://cloud-sd:8080/healthz
curl http://cloud-sd:8080/readyz
```

2. Check each engine endpoint:

```bash
curl http://cloud-sd:8080/sd/redis
curl http://cloud-sd:8080/sd/mysql
curl http://cloud-sd:8080/sd/postgres
curl http://cloud-sd:8080/sd/mongo
curl http://cloud-sd:8080/sd/node
```

3. Make sure cloud resource tags are ready:

```text
cloud_sd_scope=id1
cloud_sd_disable=false
```

If `collector.scopes` is empty, cloud-sd discovers all non-disabled resources. Set `cloud_sd_disable=true` to remove a resource from discovery.

## Common Relabel Pattern

Database and middleware exporters usually run in multi-target mode. The discovered `__address__` is the cloud resource address, for example `redis.example.com:6379`. Prometheus then scrapes the exporter and passes the cloud resource address as a query parameter.

Common pattern:

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

If the exporter expects a URI, add the scheme in `replacement`:

```yaml
relabel_configs:
  - source_labels: [__address__]
    regex: (.+)
    target_label: __param_target
    replacement: redis://$1
```

Set `instance` to the real cloud resource address, not the exporter service address.

## Redis Exporter Multi-Target Scraping

Use this when:

- `/sd/redis` returns Redis/Tair or ElastiCache addresses
- one redis_exporter probes multiple Redis instances through a `target` parameter
- Redis instances share the same auth mode, or the exporter can handle auth configuration

### Step 1: Deploy redis_exporter

Example service:

```text
redis-exporter.monitoring.svc:9121
```

If Redis requires auth, inject credentials through exporter environment variables, config files, or Secrets. Do not put passwords in cloud-sd labels.

### Step 2: Configure Prometheus

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

Final request:

```text
http://redis-exporter.monitoring.svc:9121/scrape?target=redis://redis.example.com:6379
```

### Step 3: Verify

```promql
up{job="cloud-redis"}
redis_up{job="cloud-redis"}
```

If `up=1` but `redis_up=0`, the exporter is reachable but Redis auth, networking, or Redis itself is failing.

## MySQL Exporter Multi-Target Scraping

Use this when:

- `/sd/mysql` returns RDS MySQL or Aurora MySQL addresses
- mysqld_exporter runs in `/probe` multi-target mode
- MySQL credentials are managed by the exporter config or Secrets

### Step 1: Deploy mysqld_exporter

Example service:

```text
mysqld-exporter.monitoring.svc:9104
```

Prepare an exporter auth module such as:

```text
client.cloud
```

Prometheus should pass only the target address, not database credentials.

### Step 2: Configure Prometheus

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

Final request:

```text
http://mysqld-exporter.monitoring.svc:9104/probe?auth_module=client.cloud&target=mysql.example.com:3306
```

### Step 3: Verify

```promql
up{job="cloud-mysql"}
mysql_up{job="cloud-mysql"}
```

If your exporter version does not support `/probe` or `auth_module`, upgrade it or split exporters by credential group.

## PostgreSQL Exporter Multi-Target Scraping

Use this when:

- `/sd/postgres` returns RDS PostgreSQL or Aurora PostgreSQL addresses
- postgres_exporter runs in `/probe` multi-target mode
- PostgreSQL credentials are managed by exporter auth modules or Secrets

### Step 1: Deploy postgres_exporter

Example service:

```text
postgres-exporter.monitoring.svc:9187
```

Prepare an exporter auth module such as:

```text
cloud
```

### Step 2: Configure Prometheus

Many postgres_exporter multi-target setups expect `target` to be a PostgreSQL URI. cloud-sd returns `host:port`, so relabeling can build the URI:

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

Final request:

```text
http://postgres-exporter.monitoring.svc:9187/probe?auth_module=cloud&target=postgresql://pg.example.com:5432/postgres?sslmode=disable
```

### Step 3: Verify

```promql
up{job="cloud-postgres"}
pg_up{job="cloud-postgres"}
```

If your exporter expects `target=host:port`, remove the `postgresql://` prefix and path.

## MongoDB Exporter Multi-Target Scraping

Use this when:

- `/sd/mongo` returns MongoDB or DocumentDB addresses
- mongodb_exporter accepts a target address through a query parameter
- MongoDB credentials are handled by exporter config or Secrets

### Step 1: Deploy mongodb_exporter

Example service:

```text
mongodb-exporter.monitoring.svc:9216
```

Different MongoDB exporters use different multi-target paths and parameters. The example below assumes `/scrape?target=`.

### Step 2: Configure Prometheus

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

Final request:

```text
http://mongodb-exporter.monitoring.svc:9216/scrape?target=mongodb://mongo.example.com:27017
```

### Step 3: Verify

```promql
up{job="cloud-mongo"}
mongodb_up{job="cloud-mongo"}
```

For AWS DocumentDB, configure CA, TLS, and compatibility options according to your exporter and security policy.

## Node Exporter Multi-Instance Scraping

`/sd/node` is different from database and middleware jobs. cloud-sd returns ECS/EC2 instance addresses with the Node Exporter default port `9100`, so Prometheus can scrape Node Exporter directly.

### Step 1: Run node_exporter on cloud hosts

Every ECS/EC2 host should run node_exporter and be reachable by Prometheus:

```text
<ecs-or-ec2-private-ip>:9100
```

Common deployment methods:

- systemd service
- Ansible / Terraform / cloud-init
- Kubernetes DaemonSet, although Kubernetes SD is often a better fit for Kubernetes nodes

### Step 2: Configure Prometheus

```yaml
scrape_configs:
  - job_name: cloud-node
    metrics_path: /metrics
    http_sd_configs:
      - url: http://cloud-sd:8080/sd/node
        refresh_interval: 60s
```

No exporter-service rewrite is needed because the discovered target is already the Node Exporter endpoint.

### Step 3: Verify

```promql
up{job="cloud-node"}
node_uname_info{job="cloud-node"}
node_cpu_seconds_total{job="cloud-node"}
```

cloud-sd does not filter ECS/EC2 by running state. Stopped instances remain in `/sd/node` and appear as `up=0`.

## Complete scrape_configs Example

This example assumes exporters are reachable through `monitoring` service DNS names:

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

cloud-sd returns labels such as:

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

Prometheus attaches these labels to the discovered target. You can use them in relabeling, alert rules, dashboards, and recording rules.

## Grafana Dashboard Label Compatibility

The HTTP SD label set is designed to work with TenSunS / Node Exporter style dashboard variables:

```text
vendor -> account -> group -> name -> instance
```

Mapping details:

| Label | Meaning |
|---|---|
| `vendor` | Cloud provider, for example `aliyun` or `aws` |
| `account` | Configured cloud account name, for example `accounts[].name` |
| `account_id` | Real cloud account ID resolved by STS |
| `region` | Cloud region |
| `group` | Scope tag value, usually `cloud_sd_scope` |
| `name` | Cloud resource display name |
| `iid` | Cloud resource instance ID |
| `cservice` | Service category, same value as `engine` in the MVP |
| `resource_type` | cloud-sd resource type |
| `engine` | `redis`, `mysql`, `postgres`, `mongo`, or `node` |

cloud-sd does not emit the old labels `provider`, `account_name`, `region_id`, `resource_id`, `resource_name`, or `scope`.

## Verification and Troubleshooting

### 1. Check Prometheus service discovery

Open Prometheus UI:

```text
Status -> Service Discovery
Status -> Targets
```

Check whether `cloud-redis`, `cloud-mysql`, `cloud-postgres`, `cloud-mongo`, and `cloud-node` have targets.

### 2. Check up metrics

```promql
up{job=~"cloud-.*"}
```

### 3. Check labels

```promql
count by (job, vendor, account, region, group, engine) (up{job=~"cloud-.*"})
```

### 4. Common issues

| Symptom | Common cause | Action |
|---|---|---|
| `/sd/*` returns 503 | cloud-sd is not ready or first refresh failed | Check `/readyz` and cloud-sd logs |
| No Prometheus targets | `http_sd_configs.url` is unreachable | curl cloud-sd from the Prometheus runtime |
| Target exists but `up=0` | exporter cannot probe the cloud resource | Check exporter logs, network ACLs, security groups, and database allowlists |
| `instance` is the exporter address | missing `__param_target -> instance` relabel step | add the relabel rule |
| scope filtering does not work | cloud tags are missing or tag API permissions are insufficient | check `cloud_sd_scope` and tag read permissions |
| disabled resource is still discovered | `cloud_sd_disable=true` is missing or tag reads failed | check resource tags and cloud-sd refresh logs |

### 5. Metric name reminder

Node Exporter dashboards require Node Exporter metrics such as `node_uname_info`, `node_cpu_seconds_total`, and `node_memory_*`. Database exporters expose different metric names, so label compatibility helps with identity and filtering only.
