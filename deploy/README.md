# Kubernetes Manifests

This directory contains Kubernetes manifests for cloud-sd and the exporters used by Prometheus scraping examples.

| Component | Directory | Description |
|---|---|---|
| cloud-sd | [cloud-sd](cloud-sd/) | Deploys the cloud-sd HTTP SD service, config, credentials Secret, and Service |
| Exporters | [exporters](exporters/) | Deploys Redis, MySQL, PostgreSQL, MongoDB, and Node exporters |

Apply cloud-sd:

```bash
kubectl apply -f deploy/cloud-sd/cloud-sd.yaml
```

Apply exporters:

```bash
kubectl apply -f deploy/exporters/
```

Before production use:

- replace every `CHANGE_ME` placeholder
- update `ghcr.io/ylighgh/prometheus-cloud-sd:v0.1.0` if you publish the image elsewhere
- set the enabled providers, accounts, regions, scopes, and engines in the ConfigMap
- inject cloud credentials through your secret management system
- make sure Prometheus can reach `http://cloud-sd.monitoring.svc:8080`

The default image is published by `.github/workflows/docker.yml`. Push a `v*` tag, or run the workflow manually with `image_tag=v0.1.0`, to publish `ghcr.io/ylighgh/prometheus-cloud-sd:v0.1.0`.
