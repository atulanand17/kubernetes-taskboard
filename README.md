# Cloud Native Taskboard

A small full-stack application I use to learn Kubernetes by running something real.

The application is intentionally simple: create, complete, list, and delete tasks. I did not want the business logic to become the interesting part of the project. The point of this repository is to understand what happens around an application once it is containerized and deployed to Kubernetes.

The project covers the Kubernetes fundamentals I wanted in one place:

- Pods, ReplicaSets, Deployments, and DaemonSets
- Services and cluster DNS
- ConfigMaps and Secrets
- StatefulSets and PersistentVolumeClaims
- startup, readiness, and liveness probes
- CPU/memory requests and limits
- Ingress
- Horizontal Pod Autoscaling
- PodDisruptionBudget
- Jobs and CronJobs
- Kustomize overlays
- rolling updates and rollbacks
- optional RBAC and NetworkPolicy exercises
- troubleshooting with `kubectl`

I also kept a Docker Compose path so I can prove the application works before blaming Kubernetes.

---

## What runs in the cluster?

```text
Browser
   |
   v
Ingress: taskboard.local
   |---------------------------|
   v                           v
Frontend Service             API Service
   |                           |
   v                           v
Nginx Deployment          Go API Deployment
                               |
                               v
                      PostgreSQL Service
                               |
                               v
                      PostgreSQL StatefulSet
                               |
                               v
                              PVC
```

The browser talks to a single host. Ingress routes `/` to the frontend and `/api` or `/health` to the API. The API discovers PostgreSQL through a Kubernetes Service rather than a Pod IP.

A more detailed diagram and rationale are in [docs/architecture.md](docs/architecture.md).

---

## Tech stack

| Layer | Technology | Why I chose it |
|---|---|---|
| API | Go, standard `net/http` | Small service, easy to containerize, and useful backend practice without a framework hiding the HTTP flow |
| Database | PostgreSQL | Gives the project real persistence and a reason to learn StatefulSets, Services, and PVCs |
| Frontend | HTML/CSS/JavaScript + Nginx | Enough UI to use the system without turning this into a frontend project |
| Containerization | Docker | Produces the images Kubernetes runs |
| Orchestration | Kubernetes | Main focus of the repository |
| Environment overlays | Kustomize | Keeps one base set of manifests and small dev/prod differences |

---

## Repository layout

```text
.
├── .github/workflows/ci.yml
├── api/
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   └── main_test.go
├── db/
│   └── init.sql
├── frontend/
│   ├── Dockerfile
│   ├── app.js
│   ├── index.html
│   ├── nginx.conf
│   ├── nginx-k8s.conf
│   └── styles.css
├── k8s/
│   ├── base/
│   ├── overlays/
│   │   ├── dev/
│   │   └── prod/
│   └── optional/
│       ├── daemonset.yaml
│       ├── network-policies.yaml
│       └── rbac.yaml
├── scripts/
│   ├── bootstrap-minikube.sh
│   └── smoke-test.sh
├── docs/
│   ├── architecture.md
│   ├── interview-notes.md
│   └── troubleshooting.md
├── docker-compose.yml
├── Makefile
└── README.md
```

---

# Quick start

## Prerequisites

For the full Kubernetes exercise:

- Docker
- `kubectl`
- Minikube
- `curl`
- around 4 CPUs and 6 GB RAM available to Minikube

Check the basics:

```bash
docker version
kubectl version --client
minikube version
```

The helper scripts assume a Unix-like shell (`bash`, `zsh`, or compatible shell).

---

# Step 1: Run the application without Kubernetes

I recommend doing this once before using Minikube. It gives a useful baseline: if the app fails in Compose, Kubernetes is probably not the first thing to debug.

Start the three containers:

```bash
docker compose up -d --build
```

Check status:

```bash
docker compose ps
```

Follow logs:

```bash
docker compose logs -f
```

Open:

```text
http://localhost:8080
```

Readiness check:

```bash
curl http://localhost:8080/health/ready
```

Stop everything and remove the local PostgreSQL volume:

```bash
docker compose down -v
```

---

# Step 2: Run it on Minikube

The shortest path is:

```bash
./scripts/bootstrap-minikube.sh
```

The script:

1. starts Minikube if it is not already running;
2. enables the ingress and metrics-server addons;
3. builds the API and frontend Docker images;
4. loads those images into Minikube;
5. creates the `taskboard` Namespace;
6. creates a local development Secret;
7. applies the dev Kustomize overlay;
8. waits for the main workloads to roll out.

After the deployment is ready, use a port-forward for the most portable local access path:

```bash
kubectl -n taskboard port-forward svc/taskboard-frontend 8080:8080
```

Keep that terminal open and browse to:

```text
http://localhost:8080
```

In another terminal run:

```bash
./scripts/smoke-test.sh
```

Why use port-forward in the quick start? Minikube host networking and Ingress access vary by operating system and driver. I still deploy an Ingress because learning it is part of the project, but the first successful run should not depend on a host-specific networking trick.

---

# Manual Kubernetes setup

The helper script is convenient, but doing these commands manually is where most of the learning happens.

## 1. Start a cluster

```bash
minikube start --cpus=4 --memory=6144
```

Enable metrics-server for HPA:

```bash
minikube addons enable metrics-server
```

Try enabling the Nginx ingress addon as well:

```bash
minikube addons enable ingress
```

If your Minikube driver/host combination does not support that addon or does not expose it cleanly to the host, continue with the rest of the deployment and use the port-forward path below. The Ingress object itself is still useful to inspect and discuss.

Check the cluster:

```bash
kubectl get nodes
kubectl get pods -A
```

---

## 2. Build and load the images

Build locally:

```bash
docker build -t taskboard-api:dev ./api
docker build -t taskboard-frontend:dev ./frontend
```

Load them into Minikube:

```bash
minikube image load taskboard-api:dev
minikube image load taskboard-frontend:dev
```

Why is this necessary?

The Kubernetes node needs access to the image. In a real environment I would push versioned images to a registry such as GHCR/ECR/GCR/ACR. For a local cluster, `minikube image load` avoids needing a registry.

---

## 3. Create the Namespace and Secret

Create the Namespace first:

```bash
kubectl apply -f k8s/base/namespace.yaml
```

I keep the Namespace manifest outside the Kustomize resource list on purpose. That lets `kubectl delete -k` remove the application workloads without immediately deleting the Namespace and its PVCs, which makes the storage cleanup exercise easier to observe.

Create a local password:

```bash
kubectl -n taskboard create secret generic taskboard-secret \
  --from-literal=DB_PASSWORD=taskboard-local-dev \
  --dry-run=client -o yaml | kubectl apply -f -
```

The actual password is intentionally not stored in Git.

Inspect the Secret metadata:

```bash
kubectl -n taskboard describe secret taskboard-secret
```

`describe` shows the available keys, not the decoded secret value.

---

## 4. Render the manifests before applying them

Kustomize is built into `kubectl`.

Render the development configuration:

```bash
kubectl kustomize k8s/overlays/dev
```

Nothing is created by this command. It is a good habit to inspect generated YAML before applying it.

The base contains the common objects. The dev and prod overlays make small environment-specific changes.

---

## 5. Apply the application

```bash
kubectl apply -k k8s/overlays/dev
```

Watch the Pods:

```bash
kubectl -n taskboard get pods -w
```

In another terminal:

```bash
kubectl -n taskboard get deploy,statefulset,svc,ingress,hpa,pdb,job,cronjob,pvc
```

Wait for the main workloads:

```bash
kubectl -n taskboard rollout status statefulset/taskboard-postgres
kubectl -n taskboard rollout status deployment/taskboard-api
kubectl -n taskboard rollout status deployment/taskboard-frontend
```

---

## 6. Access the application

### Portable path: port-forward the frontend Service

```bash
kubectl -n taskboard port-forward svc/taskboard-frontend 8080:8080
```

The frontend Nginx container proxies `/api` and `/health` to `taskboard-api`, so this single port-forward is enough for both the UI and API.

Test:

```bash
curl http://localhost:8080/health/ready
```

Open:

```text
http://localhost:8080
```

### Ingress path

If the Minikube ingress addon works with your host/driver, inspect the Ingress address:

```bash
kubectl -n taskboard get ingress
minikube ip
```

Map the reachable ingress IP to:

```text
taskboard.local
```

in `/etc/hosts`, then use:

```bash
curl http://taskboard.local/health/ready
```

On Docker Desktop/macOS and some Windows/WSL setups, direct access to the Minikube node network can differ from Linux. If the hostname path is not reachable, use the port-forward method above rather than changing application code.

---

# API examples

## Liveness

```bash
curl http://localhost:8080/health/live
```

Example response:

```json
{
  "status": "ok",
  "message": "hello from the Kubernetes ConfigMap"
}
```

## Readiness

```bash
curl http://localhost:8080/health/ready
```

Readiness includes a PostgreSQL connectivity check.

## Create a task

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learn Kubernetes Services"}'
```

## List tasks

```bash
curl http://localhost:8080/api/tasks
```

## Mark task 1 complete

```bash
curl -X PATCH http://localhost:8080/api/tasks/1 \
  -H 'Content-Type: application/json' \
  -d '{"completed":true}'
```

## Delete task 1

```bash
curl -X DELETE http://localhost:8080/api/tasks/1
```

---

# Kubernetes concepts demonstrated

## Namespace

File:

```text
k8s/base/namespace.yaml
```

The project lives in its own Namespace:

```bash
kubectl -n taskboard get all
```

This keeps commands and policies scoped to the application rather than mixing everything into `default`.

---

## Pods and Deployments

The frontend and API are stateless, so both use Deployments.

Inspect them:

```bash
kubectl -n taskboard get deployments
kubectl -n taskboard get pods -l app=taskboard-api
```

Delete one API Pod:

```bash
kubectl -n taskboard delete pod <api-pod-name>
```

Then watch:

```bash
kubectl -n taskboard get pods -w
```

The Pod is not “restarted” by magic. The Deployment owns a ReplicaSet whose desired state says there should be a certain number of matching Pods. The controller sees the count is too low and creates a replacement.

That reconciliation loop is one of the most important Kubernetes ideas in this project.

---

## Services and service discovery

Pod IPs are disposable. The application should not depend on them.

The API connects to PostgreSQL using:

```text
taskboard-postgres:5432
```

That hostname comes from the Service.

Inspect Services and their endpoints:

```bash
kubectl -n taskboard get svc
kubectl -n taskboard get endpoints
```

Look at the API Service:

```bash
kubectl -n taskboard describe svc taskboard-api
```

A Service selects Pods by labels and gives clients a stable virtual endpoint while the actual Pods can change.

---

## ConfigMap

Non-sensitive runtime configuration is stored in:

```text
taskboard-config
```

Inspect it:

```bash
kubectl -n taskboard get configmap taskboard-config -o yaml
```

The API reads values such as:

```text
DB_HOST
DB_PORT
DB_NAME
DB_USER
APP_MESSAGE
```

Try changing the message:

```bash
kubectl -n taskboard patch configmap taskboard-config \
  --type merge \
  -p '{"data":{"APP_MESSAGE":"configuration changed without rebuilding the image"}}'
```

The API receives ConfigMap values as environment variables. Existing Pods do not automatically reload environment variables, so restart the Deployment:

```bash
kubectl -n taskboard rollout restart deployment/taskboard-api
kubectl -n taskboard rollout status deployment/taskboard-api
```

Verify:

```bash
curl http://localhost:8080/health/live
```

This demonstrates an important separation: image contents and runtime configuration do not have to be the same thing.

---

## Secret

The database password is injected from:

```text
taskboard-secret
```

The password is not committed to this repository.

Inspect metadata:

```bash
kubectl -n taskboard describe secret taskboard-secret
```

For a real system, a plain Kubernetes Secret would not be my final secret-management strategy. I would normally integrate the cluster with a dedicated secret manager and avoid manually maintained secrets.

---

## StatefulSet

PostgreSQL is the stateful component.

Inspect it:

```bash
kubectl -n taskboard get statefulset
kubectl -n taskboard describe statefulset taskboard-postgres
```

Why not use a Deployment for PostgreSQL in this learning project?

A StatefulSet demonstrates:

- stable Pod identity;
- ordered naming such as `taskboard-postgres-0`;
- a stable relationship with persistent storage;
- `volumeClaimTemplates`.

This is useful Kubernetes knowledge even though a single PostgreSQL StatefulSet is not what I would recommend as a complete production database architecture.

---

## PersistentVolumeClaim

Inspect storage:

```bash
kubectl -n taskboard get pvc
```

Create a task in the UI, then delete the PostgreSQL Pod:

```bash
kubectl -n taskboard delete pod taskboard-postgres-0
```

Watch the Pod return:

```bash
kubectl -n taskboard get pods -w
```

Refresh the application after PostgreSQL is ready. The task should still exist because the data is stored on the volume, not in the disposable container filesystem.

This is the key lesson:

**Pod lifetime and data lifetime are different concerns.**

---

## Startup, readiness, and liveness probes

The API has three different health checks.

### Startup probe

```text
/health/live
```

Its purpose is to give the process time to start before normal liveness behavior applies.

### Readiness probe

```text
/health/ready
```

The readiness endpoint checks database connectivity.

If PostgreSQL is unavailable, the API process can still be alive, but it should not receive normal application traffic.

### Liveness probe

```text
/health/live
```

Liveness checks the API process itself and deliberately does **not** depend on PostgreSQL.

This distinction matters. A database outage should not cause Kubernetes to continuously restart otherwise healthy API processes.

Inspect probe configuration and events:

```bash
kubectl -n taskboard describe pod -l app=taskboard-api
```

---

## Requests and limits

The main containers specify CPU and memory requests/limits.

Example from the API:

```yaml
resources:
  requests:
    cpu: 50m
    memory: 32Mi
  limits:
    cpu: 250m
    memory: 128Mi
```

Inspect:

```bash
kubectl -n taskboard describe deployment taskboard-api
```

Requests tell the scheduler what a Pod needs and provide the baseline used by CPU-utilization HPA calculations. Limits constrain resource usage.

The values here are small because this is a local learning application; they are not claimed to be production sizing.

---

## Ingress

The Ingress provides one external hostname:

```text
taskboard.local
```

Routing:

```text
/        -> taskboard-frontend
/api     -> taskboard-api
/health  -> taskboard-api
```

Inspect it:

```bash
kubectl -n taskboard describe ingress taskboard
```

This is different from a Service. A Service exposes a workload inside the cluster (and can sometimes expose it externally depending on type). Ingress is an HTTP routing layer in front of Services and requires an ingress controller.

---

## HorizontalPodAutoscaler

The API has an HPA:

```bash
kubectl -n taskboard get hpa
```

Watch it:

```bash
kubectl -n taskboard get hpa -w
```

Metrics must be available:

```bash
kubectl top pods -n taskboard
```

The HPA is configured with:

- minimum API replicas: 2
- maximum API replicas: 6
- CPU target: 60%

Generate some requests:

```bash
for i in $(seq 1 2000); do
  curl -s http://localhost:8080/api/tasks >/dev/null &
done
wait
```

Watch Pods and HPA:

```bash
kubectl -n taskboard get hpa,pods -w
```

A tiny API on a laptop may finish requests too quickly to produce an obvious scaling event. That is fine. The exercise is to understand where HPA gets metrics and how it calculates desired replicas.

---

## PodDisruptionBudget

Inspect:

```bash
kubectl -n taskboard get pdb
```

The API PDB requires at least one API Pod to remain available during voluntary disruptions.

A PDB is not a general high-availability guarantee. It primarily influences voluntary eviction operations such as node drains.

---

## Job

The repository includes a one-off DB check Job.

Inspect it:

```bash
kubectl -n taskboard get jobs
```

Read its output:

```bash
kubectl -n taskboard logs job/taskboard-db-check
```

It waits for PostgreSQL, runs a SQL query, prints the task count, and exits.

Jobs are a useful pattern for work that should finish rather than remain running forever: migrations, imports, one-off maintenance, batch processing, and similar tasks.

---

## CronJob

The CronJob runs every five minutes and calls the API readiness endpoint.

```bash
kubectl -n taskboard get cronjob
```

See Jobs created by it:

```bash
kubectl -n taskboard get jobs --sort-by=.metadata.creationTimestamp
```

The check is intentionally small. The point is to learn scheduling, concurrency policy, and job history without adding another application.

---

# Kustomize: base and overlays

The common manifests live in:

```text
k8s/base
```

Environment differences live in:

```text
k8s/overlays/dev
k8s/overlays/prod
```

Render development:

```bash
kubectl kustomize k8s/overlays/dev
```

Render the production example:

```bash
kubectl kustomize k8s/overlays/prod
```

The production overlay demonstrates different:

- container image locations/tags;
- API replica count;
- PostgreSQL storage size.

The values are examples, not a production recommendation.

The useful idea is that I do not copy every manifest into `dev/` and `prod/`. The base remains the source of truth and overlays express only what changes.

---

# Rolling update exercise

Build a second API image:

```bash
docker build -t taskboard-api:v2 ./api
minikube image load taskboard-api:v2
```

Update the Deployment:

```bash
kubectl -n taskboard set image deployment/taskboard-api api=taskboard-api:v2
```

Watch:

```bash
kubectl -n taskboard rollout status deployment/taskboard-api
kubectl -n taskboard rollout history deployment/taskboard-api
```

The API uses:

```yaml
rollingUpdate:
  maxUnavailable: 0
  maxSurge: 1
```

That lets Kubernetes start a replacement before intentionally reducing the number of ready old Pods.

Rollback:

```bash
kubectl -n taskboard rollout undo deployment/taskboard-api
kubectl -n taskboard rollout status deployment/taskboard-api
```

---

# Failure exercises

Healthy demos are useful. Broken demos teach more.

## Exercise 1: delete an API Pod

```bash
kubectl -n taskboard get pods -l app=taskboard-api
kubectl -n taskboard delete pod <api-pod-name>
kubectl -n taskboard get pods -w
```

Question to answer:

> Which Kubernetes object creates the replacement and why?

---

## Exercise 2: use a bad image

```bash
kubectl -n taskboard set image deployment/taskboard-api api=does-not-exist:v1
```

Inspect:

```bash
kubectl -n taskboard get pods
kubectl -n taskboard describe pod <failing-pod>
kubectl -n taskboard get events --sort-by=.metadata.creationTimestamp
```

You should see image-pull-related failure information.

Recover:

```bash
kubectl -n taskboard rollout undo deployment/taskboard-api
```

---

## Exercise 3: make PostgreSQL unavailable

Scale it to zero:

```bash
kubectl -n taskboard scale statefulset/taskboard-postgres --replicas=0
```

Check API readiness:

```bash
curl -i http://localhost:8080/health/ready
```

Inspect API Pods:

```bash
kubectl -n taskboard get pods -l app=taskboard-api
```

The API processes should remain running but become unready.

Restore PostgreSQL:

```bash
kubectl -n taskboard scale statefulset/taskboard-postgres --replicas=1
kubectl -n taskboard rollout status statefulset/taskboard-postgres
```

---

## Exercise 4: inspect DNS

Create a temporary Pod:

```bash
kubectl -n taskboard run dns-test --rm -it --restart=Never \
  --image=busybox:1.36 -- nslookup taskboard-postgres
```

This makes Service discovery visible instead of treating it as magic.

---


# Optional DaemonSet exercise

A DaemonSet is different from a Deployment: instead of asking for an arbitrary replica count, it makes sure eligible nodes run a copy of a Pod.

Apply the node reporter:

```bash
kubectl apply -f k8s/optional/daemonset.yaml
```

Inspect it:

```bash
kubectl -n taskboard get daemonset
kubectl -n taskboard get pods -l app=taskboard-node-reporter -o wide
```

Read logs:

```bash
kubectl -n taskboard logs -l app=taskboard-node-reporter --tail=20
```

The Pod learns its own Pod name and node name through the Kubernetes **Downward API**.

On a default single-node Minikube cluster there will normally be one reporter Pod. On a multi-node cluster, the point becomes more obvious.

Clean up:

```bash
kubectl delete -f k8s/optional/daemonset.yaml
```

Typical real uses for DaemonSets include node-level log collectors, monitoring agents, and networking/storage agents.

---

# Optional NetworkPolicy exercise

NetworkPolicy is kept out of the default deployment because support and ingress behavior depend on the cluster networking implementation.

Apply it only after the normal application works:

```bash
kubectl apply -f k8s/optional/network-policies.yaml
```

Inspect:

```bash
kubectl -n taskboard get networkpolicy
```

The exercise introduces:

- default-deny ingress;
- explicit ingress-controller/internal app access;
- restricted PostgreSQL access.

If it interferes with your local cluster networking, remove it:

```bash
kubectl delete -f k8s/optional/network-policies.yaml
```

That is also a useful troubleshooting lesson: a healthy Pod and Service do not guarantee traffic is permitted.

---

# Optional RBAC exercise

Apply the example:

```bash
kubectl apply -f k8s/optional/rbac.yaml
```

It creates a ServiceAccount with read-only Pod/log access.

Test its permissions:

```bash
kubectl auth can-i list pods \
  --as=system:serviceaccount:taskboard:taskboard-reader \
  -n taskboard
```

Expected:

```text
yes
```

Now test delete permission:

```bash
kubectl auth can-i delete pods \
  --as=system:serviceaccount:taskboard:taskboard-reader \
  -n taskboard
```

Expected:

```text
no
```

This demonstrates least privilege more clearly than simply creating a ServiceAccount and never using the permission model.

---

# Troubleshooting workflow

These are the commands I want to be comfortable using without looking them up every time:

```bash
kubectl -n taskboard get pods -o wide
kubectl -n taskboard describe pod <pod-name>
kubectl -n taskboard logs <pod-name>
kubectl -n taskboard logs <pod-name> --previous
kubectl -n taskboard get events --sort-by=.metadata.creationTimestamp
kubectl -n taskboard get svc,endpoints
kubectl -n taskboard get pvc
kubectl -n taskboard top pods
kubectl -n taskboard rollout history deployment/taskboard-api
```

My debugging order is usually:

1. Is the desired workload present?
2. Is the Pod scheduled?
3. Did the container start?
4. What do `describe`, events, and logs say?
5. Is the Pod ready?
6. Does the Service actually have endpoints?
7. Does cluster DNS resolve?
8. Is configuration correct?
9. Is Ingress routing correctly?
10. Did a NetworkPolicy or resource problem block it?

There is a more detailed version in [docs/troubleshooting.md](docs/troubleshooting.md).

---

# Useful inspection commands

## See labels

```bash
kubectl -n taskboard get pods --show-labels
```

## See a Deployment as YAML

```bash
kubectl -n taskboard get deployment taskboard-api -o yaml
```

## Execute a command in PostgreSQL

```bash
kubectl -n taskboard exec -it taskboard-postgres-0 -- \
  psql -U taskboard -d taskboard
```

Inside `psql`:

```sql
SELECT * FROM tasks ORDER BY created_at DESC;
```

Exit:

```text
\q
```

## Port-forward without Ingress

If I want to remove Ingress from the debugging path:

```bash
kubectl -n taskboard port-forward svc/taskboard-api 8081:8080
```

Then:

```bash
curl http://localhost:8081/health/ready
```

This is useful for isolating whether the problem is the API/Service or the ingress layer.

---

# CI

The GitHub Actions workflow performs lightweight repository checks:

- checks Go formatting;
- downloads modules;
- runs Go tests;
- builds both Docker images;
- renders the dev and prod Kustomize overlays.

File:

```text
.github/workflows/ci.yml
```

I intentionally kept CI small. A natural next step would be publishing versioned images to GHCR and deploying from immutable image tags.

---

# Clean up

Delete the application resources:

```bash
kubectl delete -k k8s/overlays/dev
```

Check whether storage remains:

```bash
kubectl -n taskboard get pvc
```

If the PVC remains and I intentionally want to remove the data:

```bash
kubectl -n taskboard delete pvc data-taskboard-postgres-0
```

Delete the Namespace:

```bash
kubectl delete namespace taskboard
```

Stop Minikube:

```bash
minikube stop
```

Or delete the entire local cluster:

```bash
minikube delete
```

---

# What I deliberately did not call “production ready”

This repository is a learning project. I would rather state the gaps than put enterprise-looking YAML in the repo without being able to defend it.

For a real production system I would revisit at least:

- managed PostgreSQL or a well-operated Postgres operator;
- database high availability and tested restore procedures;
- TLS and real DNS;
- external secret management;
- immutable image tags;
- image vulnerability scanning and signing;
- CI/CD promotion between environments;
- centralized metrics, logs, traces, and alerting;
- stronger NetworkPolicies;
- application authentication/authorization;
- rate limiting;
- multi-zone scheduling where appropriate;
- resource sizing based on measurements rather than demo values.

That list is also useful in an interview because it separates “I know how this Kubernetes object works” from “I think one StatefulSet makes PostgreSQL production-grade.”

---

# Suggested learning order

I would work through this repository in this order:

1. Run everything with Docker Compose.
2. Build the two application images.
3. Start Minikube and inspect the node.
4. Deploy PostgreSQL and understand its Service/PVC.
5. Deploy the API and trace its configuration.
6. Deploy the frontend.
7. Understand how Services select Pods.
8. Put Ingress in front of both services.
9. Delete Pods and observe reconciliation.
10. Change a ConfigMap and perform a rollout.
11. Delete the PostgreSQL Pod and verify persistence.
12. Compare readiness and liveness during a DB outage.
13. Inspect resource requests/limits and metrics.
14. Observe HPA.
15. Perform a rolling update and rollback.
16. Inspect the Job and CronJob.
17. Run the DaemonSet exercise.
18. Add RBAC.
19. Add NetworkPolicy.
20. Work through the failure exercises without immediately reading the answer.

If I can explain each step from memory and diagnose the deliberately broken scenarios, I consider the Kubernetes fundamentals from this project learned rather than merely copied.

---

# Interview questions I should be able to answer after this project

- What creates a Pod when a Deployment is applied?
- What happens after I manually delete one of the API Pods?
- Why should an application use a Service instead of a Pod IP?
- What is the difference between ClusterIP and a headless Service?
- Why is PostgreSQL a StatefulSet here but the API a Deployment?
- What does a PVC survive?
- What happens to a PVC if a Pod is deleted?
- What is the difference between ConfigMap and Secret?
- Are Kubernetes Secrets encrypted by default simply because they are called Secrets?
- What is the difference between startup, readiness, and liveness probes?
- Why does readiness check PostgreSQL while liveness does not?
- What happens if a readiness probe fails?
- What happens if a liveness probe fails?
- What are resource requests and limits?
- How does CPU-based HPA use requests?
- What does an Ingress do that a Service does not?
- What component actually implements an Ingress?
- What is a rolling update?
- How does rollback work?
- What does a PodDisruptionBudget protect against?
- What does it *not* protect against?
- When would I use a Job versus a CronJob?
- How does Kustomize differ from copying manifests per environment?
- When would I choose a DaemonSet instead of a Deployment?
- What is the Downward API?
- What problem does RBAC solve?
- What problem does NetworkPolicy solve?
- If a Service has no endpoints, what would I inspect first?
- If a Pod is running but the application is unreachable, what would I check?
- Why would I choose a managed database in production even after learning StatefulSets?

Short answers for several of these are in [docs/interview-notes.md](docs/interview-notes.md).

---

## License

MIT
"# kubernetes-taskboard" 
