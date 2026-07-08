# Exporter Prometheus YAML Snippets

These files provide ready-to-copy Prometheus `scrape_configs` for cloud-sd HTTP SD endpoints.

Kubernetes install manifests for the exporters live in [`deploy/exporters`](../../../deploy/exporters/).

| Exporter | Scrape config | Install manifest | cloud-sd endpoint | Job name |
|---|---|---|---|---|
| Redis Exporter | [cloud-redis.yaml](cloud-redis.yaml) | [redis-exporter.yaml](../../../deploy/exporters/redis-exporter.yaml) | `/sd/redis` | `cloud-redis` |
| MySQL Exporter | [cloud-mysql.yaml](cloud-mysql.yaml) | [mysql-exporter.yaml](../../../deploy/exporters/mysql-exporter.yaml) | `/sd/mysql` | `cloud-mysql` |
| PostgreSQL Exporter | [cloud-postgres.yaml](cloud-postgres.yaml) | [postgres-exporter.yaml](../../../deploy/exporters/postgres-exporter.yaml) | `/sd/postgres` | `cloud-postgres` |
| MongoDB Exporter | [cloud-mongo.yaml](cloud-mongo.yaml) | [mongodb-exporter.yaml](../../../deploy/exporters/mongodb-exporter.yaml) | `/sd/mongo` | `cloud-mongo` |
| Node Exporter | [cloud-node.yaml](cloud-node.yaml) | [node-exporter.yaml](../../../deploy/exporters/node-exporter.yaml) | `/sd/node` | `cloud-node` |

Each file is a standalone Prometheus YAML fragment with a top-level `scrape_configs` key.

If your `prometheus.yml` already has `scrape_configs`, copy only the `- job_name: ...` item into that list.

If you use Prometheus Operator or a Helm chart with `additionalScrapeConfigs`, copy only the job item as well.
