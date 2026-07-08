# cloud-sd Kubernetes Deployment

[cloud-sd.yaml](cloud-sd.yaml) installs cloud-sd into the `monitoring` namespace.

It includes:

- `Namespace`
- `ServiceAccount`
- `Secret` for cloud credentials
- `ConfigMap` for `cloud-sd.yaml`
- `Deployment`
- `Service`

Apply:

```bash
kubectl apply -f deploy/cloud-sd/cloud-sd.yaml
```

Check:

```bash
kubectl -n monitoring rollout status deploy/cloud-sd
kubectl -n monitoring get pods -l app.kubernetes.io/name=cloud-sd
kubectl -n monitoring port-forward svc/cloud-sd 8080:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/sd/redis
```

The default Service DNS is:

```text
cloud-sd.monitoring.svc:8080
```

If Prometheus runs in the same namespace, `http://cloud-sd:8080/sd/redis` is enough. From another namespace, use `http://cloud-sd.monitoring.svc:8080/sd/redis`.

Before production use:

- replace every `CHANGE_ME` placeholder in the Secret
- review the ConfigMap accounts, regions, scopes, and enabled engines
- replace the image if you publish it outside `ghcr.io/ylighgh/cloud-sd`
- prefer ExternalSecret, SealedSecret, or your cloud secret manager for credentials
- restrict network access so only Prometheus and trusted operators can call cloud-sd
