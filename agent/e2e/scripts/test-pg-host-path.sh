#!/bin/bash
set -euo pipefail

ARTIFACTS="/opt/agent/artifacts"
AGENT="/tmp/test-agent"

# Cleanup from previous runs
pkill -f "test-agent" 2>/dev/null || true
for i in $(seq 1 20); do
  pgrep -f "test-agent" > /dev/null 2>&1 || break
  sleep 0.5
done
pkill -9 -f "test-agent" 2>/dev/null || true
sleep 0.5
rm -f "$AGENT" "$AGENT.update" databasus.lock databasus.log databasus.log.old databasus.json 2>/dev/null || true

# Copy agent binary
cp "$ARTIFACTS/agent-v1" "$AGENT"
chmod +x "$AGENT"

# Verify pg_basebackup is in PATH
if ! which pg_basebackup > /dev/null 2>&1; then
  echo "FAIL: pg_basebackup not found in PATH (test setup issue)"
  exit 1
fi

# Run start with --skip-update and pg-type=host
echo "Running agent start (pg_basebackup in PATH)..."
OUTPUT=$("$AGENT" start \
  --skip-update \
  --databasus-host http://e2e-mock-server:4050 \
  --db-id test-db-id \
  --token test-token \
  --pg-host e2e-postgres \
  --pg-port 5432 \
  --pg-user testuser \
  --pg-password testpassword \
  --pg-wal-dir /tmp/wal \
  --pg-type host 2>&1)

EXIT_CODE=$?
echo "$OUTPUT"

if [ "$EXIT_CODE" -ne 0 ]; then
  echo "FAIL: Agent exited with code $EXIT_CODE"
  exit 1
fi

if ! echo "$OUTPUT" | grep -q "pg_basebackup verified"; then
  echo "FAIL: Expected output to contain 'pg_basebackup verified'"
  exit 1
fi

if ! echo "$OUTPUT" | grep -q "PostgreSQL connection verified"; then
  echo "FAIL: Expected output to contain 'PostgreSQL connection verified'"
  exit 1
fi

echo "pg_basebackup found in PATH and DB connection verified"
