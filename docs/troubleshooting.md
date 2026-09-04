# Troubleshooting playbook

The point of this file is to practice a repeatable debugging order instead of guessing.

## 1. Start with workload status

```bash
kubectl -n taskboard get pods -o wide
kubectl -n taskboard get deploy,statefulset,svc,ingress
```

## 2. Inspect a failing Pod

```bash
kubectl -n taskboard describe pod <pod-name>
kubectl -n taskboard logs <pod-name>
kubectl -n taskboard logs <pod-name> --previous
```

Typical signals:

- `ImagePullBackOff`: image name, registry authentication, or local image-loading problem.
- `CrashLoopBackOff`: the process starts and repeatedly exits.
- `Pending`: often scheduling, PVC provisioning, or resource pressure.
- readiness failing: the process is running but Kubernetes should not send it traffic.

## 3. Check Services and endpoints

```bash
kubectl -n taskboard get svc taskboard-api -o yaml
kubectl -n taskboard get endpoints taskboard-api
kubectl -n taskboard get pods -l app=taskboard-api --show-labels
```

A Service with no endpoints usually means its selector does not match ready Pods.

## 4. Check DNS from inside the cluster

```bash
kubectl -n taskboard run dns-test --rm -it --restart=Never \
  --image=busybox:1.36 -- nslookup taskboard-postgres
```

## 5. Check configuration

```bash
kubectl -n taskboard get configmap taskboard-config -o yaml
kubectl -n taskboard describe secret taskboard-secret
```

`describe secret` shows metadata and keys without printing the secret value.

## 6. Check persistent storage

```bash
kubectl -n taskboard get pvc,pv
kubectl -n taskboard describe pvc data-taskboard-postgres-0
```

## 7. Check Ingress

```bash
kubectl -n taskboard describe ingress taskboard
kubectl -n ingress-nginx get pods
```

For Minikube, confirm the ingress addon is enabled and `taskboard.local` resolves to `minikube ip`.
