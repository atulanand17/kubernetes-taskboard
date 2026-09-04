#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "Checking readiness..."
curl --fail --silent --show-error "$BASE_URL/health/ready"
echo

echo "Creating a task..."
curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -d '{"title":"smoke test from scripts/smoke-test.sh"}' \
  "$BASE_URL/api/tasks"
echo

echo "Listing tasks..."
curl --fail --silent --show-error "$BASE_URL/api/tasks"
echo
