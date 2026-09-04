APP_NAME := taskboard
NAMESPACE := taskboard

.PHONY: test compose-up compose-down compose-logs k8s-secret k8s-apply k8s-delete smoke

test:
	cd api && go test ./...

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down -v

compose-logs:
	docker compose logs -f

k8s-secret:
	kubectl apply -f k8s/base/namespace.yaml
	kubectl -n $(NAMESPACE) create secret generic taskboard-secret \
		--from-literal=DB_PASSWORD=taskboard-local-dev \
		--dry-run=client -o yaml | kubectl apply -f -

k8s-apply:
	kubectl apply -k k8s/overlays/dev

k8s-delete:
	kubectl delete -k k8s/overlays/dev --ignore-not-found

smoke:
	./scripts/smoke-test.sh
