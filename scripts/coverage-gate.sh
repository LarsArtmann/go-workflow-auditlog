#!/usr/bin/env sh
# Coverage gate for go-workflow-auditlog — mirrors the CI coverage job.
#
# Runs the race-enabled test suite with a coverage profile and fails if
# coverage drops below 92%.
#
# Usage (from the repo root, in the devShell):
#   scripts/coverage-gate.sh

set -e

go test -race -count=1 -coverprofile=cover.out -covermode=atomic -coverpkg=./ ./...

coverage=$(go tool cover -func=cover.out | awk '/^total/ {print $NF}' | tr -d '%')
echo "Total coverage: ${coverage}%"

threshold=92
if awk "BEGIN {exit !($coverage < $threshold)}"; then
  echo "❌ Coverage ${coverage}% is below ${threshold}%" >&2
  exit 1
fi

echo "✓ Coverage ${coverage}% meets the ${threshold}% gate"
