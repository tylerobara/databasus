#!/bin/bash
set -euo pipefail

ARTIFACTS="/opt/agent/artifacts"
AGENT="/tmp/test-agent"

# Ensure mock server returns v2.0.0
curl -sf -X POST http://e2e-mock-server:4050/mock/set-version \
  -H "Content-Type: application/json" \
  -d '{"version":"v2.0.0"}'

# Copy v1 binary to writable location
cp "$ARTIFACTS/agent-v1" "$AGENT"
chmod +x "$AGENT"

# Verify initial version
VERSION=$("$AGENT" version)
if [ "$VERSION" != "v1.0.0" ]; then
  echo "FAIL: Expected initial version v1.0.0, got $VERSION"
  exit 1
fi
echo "Initial version: $VERSION"

# Run start — agent will:
# 1. Fetch version from mock (v2.0.0 != v1.0.0)
# 2. Download v2 binary from mock
# 3. Replace itself on disk
# 4. Re-exec with same args
# 5. Re-exec'd v2 fetches version (v2.0.0 == v2.0.0) → skips update
# 6. Proceeds to start → verifies pg_basebackup + DB → exits 0 (stub)
echo "Running agent start (expecting upgrade v1 -> v2)..."
OUTPUT=$("$AGENT" start \
  --databasus-host http://e2e-mock-server:4050 \
  --db-id test-db-id \
  --token test-token \
  --pg-host e2e-postgres \
  --pg-port 5432 \
  --pg-user testuser \
  --pg-password testpassword \
  --wal-dir /tmp/wal \
  --pg-type host 2>&1) || true

echo "$OUTPUT"

# Verify binary on disk is now v2
VERSION=$("$AGENT" version)
if [ "$VERSION" != "v2.0.0" ]; then
  echo "FAIL: Expected upgraded version v2.0.0, got $VERSION"
  exit 1
fi

echo "Binary upgraded successfully to $VERSION"
