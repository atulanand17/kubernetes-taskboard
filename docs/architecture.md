# Architecture

```text
Browser
  |
  v
Ingress (taskboard.local)
  |-------------------------|
  v                         v
Frontend Service           API Service
  |                         |
  v                         v
Nginx Pods              Go API Pods
                            |
                            v
                    Headless PostgreSQL Service
                            |
                            v
                    PostgreSQL StatefulSet
                            |
                            v
                    PersistentVolumeClaim
```

## Why these components?

- **Frontend Deployment**: static content is stateless, so replicas are cheap to replace.
- **API Deployment**: the application tier is stateless and can scale horizontally.
- **PostgreSQL StatefulSet**: demonstrates stable identity and persistent storage. It is intentionally a learning setup; for a real production system I would normally use a managed database or a mature database operator.
- **Services**: provide stable discovery names while Pod IPs come and go.
- **Ingress**: provides one HTTP entry point and routes `/api` separately from `/`.
- **HPA**: demonstrates resource-metric-based scaling for the API.
- **PDB**: protects a minimum amount of API availability during voluntary disruptions.
- **ConfigMap / Secret**: separate runtime configuration from container images.
- **Job / CronJob**: demonstrate one-time and scheduled workloads.
