# prometheus-cloud-sd Kubernetes Deployment

[prometheus-cloud-sd.yaml](prometheus-cloud-sd.yaml) installs prometheus-cloud-sd into the `monitoring` namespace.

It includes:

- `Namespace`
- `ServiceAccount`
- `Secret` for cloud credentials
- `ConfigMap` for `prometheus-cloud-sd.yaml`
- `Deployment`
- `Service`

Apply:

```bash
kubectl apply -f deploy/prometheus-cloud-sd/prometheus-cloud-sd.yaml
```

Check:

```bash
kubectl -n monitoring rollout status deploy/prometheus-cloud-sd
kubectl -n monitoring get pods -l app.kubernetes.io/name=prometheus-cloud-sd
kubectl -n monitoring port-forward svc/prometheus-cloud-sd 8080:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/sd/redis
```

The default Service DNS is:

```text
prometheus-cloud-sd.monitoring.svc:8080
```

If Prometheus runs in the same namespace, `http://prometheus-cloud-sd:8080/sd/redis` is enough. From another namespace, use `http://prometheus-cloud-sd.monitoring.svc:8080/sd/redis`.

Before production use:

- replace every `CHANGE_ME` placeholder in the Secret
- review the ConfigMap accounts, regions, scopes, and enabled engines
- replace the image if you publish it outside `ghcr.io/ylighgh/prometheus-cloud-sd`
- prefer ExternalSecret, SealedSecret, or your cloud secret manager for credentials
- restrict network access so only Prometheus and trusted operators can call prometheus-cloud-sd
