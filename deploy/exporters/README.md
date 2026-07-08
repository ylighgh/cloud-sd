# Exporter Kubernetes Manifests

These manifests install the exporters used by the Prometheus examples in `docs/prometheus/exporters/`.

| Exporter | Install manifest | Prometheus scrape config |
|---|---|---|
| Redis Exporter | [redis-exporter.yaml](redis-exporter.yaml) | [cloud-redis.yaml](../../docs/prometheus/exporters/cloud-redis.yaml) |
| MySQL Exporter | [mysql-exporter.yaml](mysql-exporter.yaml) | [cloud-mysql.yaml](../../docs/prometheus/exporters/cloud-mysql.yaml) |
| PostgreSQL Exporter | [postgres-exporter.yaml](postgres-exporter.yaml) | [cloud-postgres.yaml](../../docs/prometheus/exporters/cloud-postgres.yaml) |
| MongoDB Exporter | [mongodb-exporter.yaml](mongodb-exporter.yaml) | [cloud-mongo.yaml](../../docs/prometheus/exporters/cloud-mongo.yaml) |
| Node Exporter | [node-exporter.yaml](node-exporter.yaml) | [cloud-node.yaml](../../docs/prometheus/exporters/cloud-node.yaml) |

Apply one exporter:

```bash
kubectl apply -f deploy/exporters/redis-exporter.yaml
```

Apply all exporters:

```bash
kubectl apply -f deploy/exporters/
```

Before production use:

- replace every `CHANGE_ME` placeholder
- pin exporter images to reviewed versions or image digests
- move credentials to your secret management system
- adjust TLS, CA, database name, and auth modules for your environment
- verify that Prometheus can reach the exporter Services and the cloud resource endpoints

The default namespace is `monitoring`. If you use another namespace, update both the Kubernetes manifests and the Prometheus scrape YAML service addresses.

## Exporter projects

- [redis_exporter](https://github.com/oliver006/redis_exporter)
- [mysqld_exporter](https://github.com/prometheus/mysqld_exporter)
- [postgres_exporter](https://github.com/prometheus-community/postgres_exporter)
- [mongodb_exporter](https://github.com/percona/mongodb_exporter)
- [node_exporter](https://github.com/prometheus/node_exporter)
