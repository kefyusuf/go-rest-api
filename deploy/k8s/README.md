# Kubernetes deployment

The `deploy/k8s/` directory carries a small Kustomize bundle. Apply
the whole thing with:

```bash
kubectl apply -k deploy/k8s
```

It creates:

| File | Resource | Purpose |
| --- | --- | --- |
| `00-namespace.yaml` | Namespace | `go-rest-api` namespace with a label |
| `10-configmap.yaml` | ConfigMap | non-secret config (timeouts, rate limits, cache TTL) |
| `20-deployment.yaml` | Deployment + ServiceAccount | 2 replicas, RollingUpdate, non-root, `seccompProfile: RuntimeDefault`, `readOnlyRootFilesystem: true`, startup/liveness/readiness probes against `/health/live` and `/health/ready`, `topologySpreadConstraints` across zones |
| `21-service.yaml` | Service + HorizontalPodAutoscaler | ClusterIP on port 80 -> 8080, HPA 2-10 replicas on CPU/memory |
| `30-ingress.yaml` | Ingress + NetworkPolicy | nginx ingress with TLS via cert-manager, default-deny NetworkPolicy that only allows ingress from the ingress-nginx namespace and the postgres/redis pods, and egress to the same plus DNS |
| `40-poddisruptionbudget.yaml` | PodDisruptionBudget | `minAvailable: 1` so voluntary disruptions do not take every pod down at once |
| `50-servicemonitor.yaml` | ServiceMonitor | tells Prometheus Operator to scrape `/metrics` every 15s |

The Secret is deliberately not part of the bundle: it carries real
credentials, so `11-secret.yaml` is git-ignored and only the tracked
`11-secret.yaml.example` template is in the repository. Create the
Secret before applying the bundle:

```bash
cp deploy/k8s/11-secret.yaml.example deploy/k8s/11-secret.yaml
# edit 11-secret.yaml: replace the placeholder JWT_SECRET (generate
# with openssl rand -base64 48), DATABASE_URL and REDIS_URL
kubectl apply -f deploy/k8s/11-secret.yaml
```

The Deployment reads `JWT_SECRET`, `DATABASE_URL` and `REDIS_URL`
from that one secret via `envFrom`.

Override the image tag in `kustomization.yaml` (`images[].newTag`)
to point at the release you want. The default `v0.1.0` is a
placeholder; CI builds `ghcr.io/your-org/go-rest-api:<version>`
from the `release.yml` workflow.

`NetworkPolicy` follows the principle of least privilege: the pod
is reachable only from the ingress controller and the data pods,
and the pod itself can only reach the data pods and DNS. Until the
policy is installed the cluster default (allow-all) applies, so
apply this bundle before exposing the service publicly.

For values you want to override per environment (image tag,
replica count, rate limit, ingress host), wrap this bundle in a
Helm chart or a Kustomize overlay. The `deploy/helm/` directory
ships a small Helm chart for the same set of resources.
