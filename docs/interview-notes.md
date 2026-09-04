# Interview notes

These are deliberately short. They are prompts for explaining decisions in your own words.

## Why a Deployment for the API?

The API is stateless. Any replica can serve any request, so a Deployment gives replica management, self-healing, rolling updates, and easy horizontal scaling.

## Why a StatefulSet for PostgreSQL?

It demonstrates persistent identity and storage. The database Pod gets a stable ordinal and a PVC that survives Pod recreation. For production, I would normally prefer a managed PostgreSQL service or a mature database operator.

## Readiness vs liveness

- **Readiness** asks: should this Pod receive traffic right now?
- **Liveness** asks: is the process unhealthy enough that Kubernetes should restart it?
- **Startup probe** protects a slow-starting process from premature liveness failures.

The API readiness endpoint checks PostgreSQL. The liveness endpoint deliberately does not. A temporary DB outage should remove an API Pod from service; it should not trigger a restart loop of a healthy process.

## Why requests and limits?

Requests influence scheduling and provide a baseline for CPU-utilization HPA calculations. Limits cap resource consumption. Together they make resource behavior explicit.

## Why is NetworkPolicy optional here?

Enforcement depends on the cluster CNI and ingress implementation. Keeping it optional makes the base example portable while still giving a practical default-deny exercise.

## What would change for production?

Managed PostgreSQL, TLS, external secret management, immutable registry tags, CI/CD, image scanning/signing, centralized logs/metrics/traces, tested backups, stricter NetworkPolicies, and multi-zone availability where appropriate.
