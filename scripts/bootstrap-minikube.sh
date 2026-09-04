#!/usr/bin/env sh
set -eu

minikube status >/dev/null 2>&1 || minikube start --cpus=4 --memory=6144

# HPA needs resource metrics.
minikube addons enable metrics-server

# Ingress support/access differs across Minikube drivers and host OSes.
# The application does not depend on this succeeding because the README also
# provides a port-forward path that works without Ingress.
if ! minikube addons enable ingress; then
  echo "Ingress addon could not be enabled on this Minikube setup; continuing."
fi

docker build -t taskboard-api:dev ./api
docker build -t taskboard-frontend:dev ./frontend
minikube image load taskboard-api:dev
minikube image load taskboard-frontend:dev

kubectl apply -f k8s/base/namespace.yaml
kubectl -n taskboard create secret generic taskboard-secret \
  --from-literal=DB_PASSWORD=taskboard-local-dev \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -k k8s/overlays/dev
kubectl -n taskboard rollout status statefulset/taskboard-postgres --timeout=180s
kubectl -n taskboard rollout status deployment/taskboard-api --timeout=180s
kubectl -n taskboard rollout status deployment/taskboard-frontend --timeout=180s

printf '\nApplication deployed. For the most portable local access path run:\n'
printf 'kubectl -n taskboard port-forward svc/taskboard-frontend 8080:8080\n'
printf '\nThen open: http://localhost:8080\n'
printf '\nIngress is also installed as a separate exercise; see README.md for driver-specific notes.\n'
