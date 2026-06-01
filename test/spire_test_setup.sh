#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# start-spire.sh
#
# Starts the Admin SPIRE Server + Agent (admin.app) and two federated test
# servers + agents (domain-a.test, domain-b.test), all using https_web for
# their federation bundle endpoints.
#
# A shared local CA is generated once and referenced directly in each server's
# TLS config — no system trust store modification required.
#
# Usage:
#   ./start-spire.sh <data_directory_path> [OPTIONS]
#
# Options:
#   --admin-server-port   Admin server gRPC port        (default: 8081)
#   --admin-bundle-port   Admin bundle endpoint port    (default: 8443)
#   --server-port-a       Server A gRPC port            (default: 8082)
#   --server-port-b       Server B gRPC port            (default: 8083)
#   --bundle-port-a       Server A bundle endpoint port (default: 8444)
#   --bundle-port-b       Server B bundle endpoint port (default: 8445)
# =============================================================================

# ── Validate required positional arg ─────────────────────────────────────────

if [ -z "${1:-}" ]; then
    echo "Usage: $0 <data_directory_path> [OPTIONS]"
    exit 1
fi

DATA_ROOT="$1"
shift

# ── Defaults ──────────────────────────────────────────────────────────────────

ADMIN_SERVER_PORT=8081
ADMIN_BUNDLE_PORT=8443
SERVER_PORT_A=8082
SERVER_PORT_B=8083
BUNDLE_PORT_A=8444
BUNDLE_PORT_B=8445

# ── Argument parsing ──────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --admin-server-port) ADMIN_SERVER_PORT="$2"; shift 2 ;;
    --admin-bundle-port) ADMIN_BUNDLE_PORT="$2"; shift 2 ;;
    --server-port-a)     SERVER_PORT_A="$2";     shift 2 ;;
    --server-port-b)     SERVER_PORT_B="$2";     shift 2 ;;
    --bundle-port-a)     BUNDLE_PORT_A="$2";     shift 2 ;;
    --bundle-port-b)     BUNDLE_PORT_B="$2";     shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Derived paths ─────────────────────────────────────────────────────────────

ADMIN_DIR="${DATA_ROOT}/admin"
TEST_DIR="${DATA_ROOT}/test"
BUNDLES_DIR="${TEST_DIR}/trust_bundles"

# Ensure all paths are absolute
for var in ADMIN_DIR TEST_DIR BUNDLES_DIR; do
  val="${!var}"
  if [[ ! "$val" = /* ]]; then
    printf -v "$var" '%s/%s' "$(pwd)" "$val"
  fi
done

ADMIN_SOCK="${ADMIN_DIR}/admin-server.sock"

# =============================================================================
# SHARED HELPERS
# =============================================================================

log() { echo "[$(date '+%H:%M:%S')] $*"; }
die() { echo "[ERROR] $*" >&2; exit 1; }

check_deps() {
  command -v spire-server &>/dev/null || die "spire-server not found in PATH"
  command -v spire-agent  &>/dev/null || die "spire-agent not found in PATH"
  command -v openssl      &>/dev/null || die "openssl not found in PATH"
}

wait_for_server() {
  local sock="$1" label="$2"
  local output
  log "Waiting for ${label} to be ready..."
  for i in $(seq 1 20); do
    output=$(spire-server bundle show -socketPath "$sock" 2>/dev/null) || true
    if [[ -n "$output" ]]; then
      return 0
    fi
    sleep 1
  done
  die "${label} did not become ready — check logs in ${DATA_ROOT}"
}

# =============================================================================
# ADMIN — admin.app server + agent
# =============================================================================

cleanup_admin() {
  log "Cleaning up previous admin data..."
  killall spire-server spire-agent 2>/dev/null || true
  rm -rf "${ADMIN_DIR}/admin-server"* "${ADMIN_DIR}/admin-agent"*
  mkdir -p "${ADMIN_DIR}"
}

write_admin_server_config() {
  cat <<EOF > "${ADMIN_DIR}/admin-server.conf"
server {
    bind_address = "127.0.0.1"
    bind_port = ${ADMIN_SERVER_PORT}
    socket_path = "${ADMIN_SOCK}"
    trust_domain = "admin.app"
    admin_ids = ["spiffe://admin.app/spire_admin"]
    data_dir = "${ADMIN_DIR}/admin-server-data"
    log_level = "INFO"
    federation {
        bundle_endpoint {
            address = "127.0.0.1"
            port = ${ADMIN_BUNDLE_PORT}
        }
        federates_with "domain-a.test" {
            bundle_endpoint_url = "https://127.0.0.1:${BUNDLE_PORT_A}"
            bundle_endpoint_profile "https_spiffe" {
                endpoint_spiffe_id = "spiffe://domain-a.test/spire/server"
            }
        }
        federates_with "domain-b.test" {
            bundle_endpoint_url = "https://127.0.0.1:${BUNDLE_PORT_B}"
            bundle_endpoint_profile "https_spiffe" {
                endpoint_spiffe_id = "spiffe://domain-b.test/spire/server"
            }
        }
    }
}
plugins {
    DataStore "sql" { plugin_data { database_type = "sqlite3", connection_string = "${ADMIN_DIR}/admin-server-data/datastore.sqlite3" } }
    KeyManager "disk" { plugin_data { keys_path = "${ADMIN_DIR}/admin-server-data/keys.json" } }
    NodeAttestor "join_token" { plugin_data {} }
}
EOF
}

write_admin_agent_config() {
  cat <<EOF > "${ADMIN_DIR}/admin-agent.conf"
agent {
    data_dir = "${ADMIN_DIR}/admin-agent-data"
    log_level = "INFO"
    server_address = "127.0.0.1"
    server_port = ${ADMIN_SERVER_PORT}
    socket_path = "${ADMIN_DIR}/admin-agent.sock"
    trust_domain = "admin.app"
    trust_bundle_path = "${BUNDLES_DIR}/admin-server.pem"
}
plugins {
    NodeAttestor "join_token" { plugin_data {} }
    KeyManager "disk" { plugin_data { directory = "${ADMIN_DIR}/admin-agent-data" } }
    WorkloadAttestor "unix" { plugin_data {} }
}
EOF
}

start_admin_server() {
  log "Starting Admin SPIRE Server..."
  spire-server run -config "${ADMIN_DIR}/admin-server.conf" \
    > "${ADMIN_DIR}/admin-server.log" 2>&1 &
  PID_ADMIN=$!

  wait_for_server "$ADMIN_SOCK" "Admin Server (admin.app)"
}

start_admin_agent() {
  log "Starting Admin Agent..."
  local token
  token=$(spire-server token generate \
    -socketPath "$ADMIN_SOCK" \
    -spiffeID   "spiffe://admin.app/admin-agent" \
    | awk '{print $2}')

  log "Creating workload entry for spire_admin..."
  spire-server entry create \
    -socketPath "$ADMIN_SOCK" \
    -parentID   spiffe://admin.app/admin-agent \
    -spiffeID   spiffe://admin.app/spire_admin \
    -selector   "unix:uid:$(id -u)" \
    -admin \
    -federatesWith spiffe://domain-a.test \
    -federatesWith spiffe://domain-b.test \
    > /dev/null

  spire-agent run \
    -config    "${ADMIN_DIR}/admin-agent.conf" \
    -joinToken "$token" \
    > "${ADMIN_DIR}/admin-agent.log" 2>&1 &
}



# =============================================================================
# TEST — domain-a.test + domain-b.test servers and agents
# =============================================================================

cleanup_test() {
  log "Cleaning up previous test data..."
  rm -rf "${TEST_DIR}/test-server-a"* \
         "${TEST_DIR}/test-server-b"* \
         "${TEST_DIR}/test-agent-a"*  \
         "${TEST_DIR}/test-agent-b"*  \
         "${BUNDLES_DIR}"
  mkdir -p "${TEST_DIR}" "${BUNDLES_DIR}"
}

write_test_server_config() {
  local letter="$1" server_port="$2" bundle_port="$3"
  local domain="domain-${letter}.test"

  cat <<EOF > "${TEST_DIR}/test-server-${letter}.conf"
server {
    bind_address = "127.0.0.1"
    bind_port = ${server_port}
    socket_path = "${TEST_DIR}/test-server-${letter}.sock"
    trust_domain = "${domain}"
    admin_ids = ["spiffe://admin.app/spire_admin"]
    data_dir = "${TEST_DIR}/test-server-${letter}-data"
    log_level = "INFO"
    federation {
        bundle_endpoint {
            address = "127.0.0.1"
            port = ${bundle_port}
        }
        federates_with "admin.app" {
            bundle_endpoint_url = "https://127.0.0.1:${ADMIN_BUNDLE_PORT}"
            bundle_endpoint_profile "https_spiffe" {
                endpoint_spiffe_id = "spiffe://admin.app/spire/server"
            }
        }
    }
}
plugins {
    DataStore "sql" { plugin_data { database_type = "sqlite3", connection_string = "${TEST_DIR}/test-server-${letter}-data/datastore.sqlite3" } }
    KeyManager "disk" { plugin_data { keys_path = "${TEST_DIR}/test-server-${letter}-data/keys.json" } }
    NodeAttestor "join_token" { plugin_data {} }
}
EOF
}

write_test_agent_config() {
  local letter="$1" server_port="$2"
  local domain="domain-${letter}.test"

  cat <<EOF > "${TEST_DIR}/test-agent-${letter}.conf"
agent {
    data_dir = "${TEST_DIR}/test-agent-${letter}-data"
    log_level = "INFO"
    server_address = "127.0.0.1"
    server_port = ${server_port}
    socket_path = "${TEST_DIR}/test-agent-${letter}.sock"
    trust_domain = "${domain}"
    trust_bundle_path = "${BUNDLES_DIR}/test-server-${letter}.pem"
}
plugins {
    NodeAttestor "join_token" { plugin_data {} }
    KeyManager "disk" { plugin_data { directory = "${TEST_DIR}/test-agent-${letter}-data" } }
    WorkloadAttestor "unix" { plugin_data {} }
}
EOF
}

start_test_servers() {
  log "Starting federated SPIRE Servers..."
  spire-server run -config "${TEST_DIR}/test-server-a.conf" \
    > "${TEST_DIR}/test-server-a.log" 2>&1 &
  spire-server run -config "${TEST_DIR}/test-server-b.conf" \
    > "${TEST_DIR}/test-server-b.log" 2>&1 &

  wait_for_server "${TEST_DIR}/test-server-a.sock" "Server A (domain-a.test)"
  wait_for_server "${TEST_DIR}/test-server-b.sock" "Server B (domain-b.test)"
}

generate_trust_bundle() {
  spire-server bundle show -socketPath "${TEST_DIR}/test-server-a.sock" \
    > "${BUNDLES_DIR}/test-server-a.pem"
  spire-server bundle show -socketPath "${TEST_DIR}/test-server-b.sock" \
    > "${BUNDLES_DIR}/test-server-b.pem"
  spire-server bundle show -socketPath "${ADMIN_DIR}/admin-server.sock" \
    > "${BUNDLES_DIR}/admin-server.pem"

}

bootstrap_federation() {
  # test-server-a and test-server-b have `federates_with "admin.app"` in their
  # config and share the same local CA for TLS, so they fetch admin.app's
  # bundle automatically on startup — no manual seeding needed in that direction.
  #
  # Admin server has no `federates_with` entries for the test domains, so it
  # has no way to auto-discover them. We push their bundles in manually once.
  log "Pushing admin server bundles into domain-a.test and domain-b.test ..."
  spire-server bundle set \
    -socketPath "${TEST_DIR}/test-server-a.sock" \
    -id         "spiffe://admin.app" \
    -format     pem \
    -path       "${BUNDLES_DIR}/admin-server.pem"

  spire-server bundle set \
    -socketPath "${TEST_DIR}/test-server-b.sock" \
    -id         "spiffe://admin.app" \
    -format     pem \
    -path       "${BUNDLES_DIR}/admin-server.pem"

  log "Pushing domain-a.test and domain-b.test bundles into admin server..."
  spire-server bundle set \
    -socketPath "$ADMIN_SOCK" \
    -id         "spiffe://domain-a.test" \
    -format     pem \
    -path       "${BUNDLES_DIR}/test-server-a.pem"

  spire-server bundle set \
    -socketPath "$ADMIN_SOCK" \
    -id         "spiffe://domain-b.test" \
    -format     pem \
    -path       "${BUNDLES_DIR}/test-server-b.pem"
}

start_test_agents() {
  log "Registering workloads and starting test agents..."
  for letter in a b; do
    local sock="${TEST_DIR}/test-server-${letter}.sock"
    local domain="domain-${letter}.test"

    spire-server entry create \
      -socketPath "$sock" \
      -parentID   "spiffe://$domain/test-agent-${letter}" \
      -spiffeID   "spiffe://$domain/workload-${letter}" \
      -selector   "unix:uid:$(id -u)" \
      > /dev/null

    local token
    token=$(spire-server token generate \
      -socketPath "$sock" \
      -spiffeID   "spiffe://$domain/test-agent-${letter}" \
      | awk '{print $2}')

    spire-agent run \
      -config    "${TEST_DIR}/test-agent-${letter}.conf" \
      -joinToken "$token" \
      > "${TEST_DIR}/test-agent-${letter}.log" 2>&1 &
  done
}

setup_admin_server() {
  log "=== Setting up Admin (admin.app) ==="
  write_admin_server_config
  start_admin_server
}

setup_admin_agent() {
  log "=== Setting up Admin Agent ==="
  write_admin_agent_config
  start_admin_agent
}

setup_test_server() {
  log "=== Setting up Test Servers (domain-a.test, domain-b.test) ==="
  write_test_server_config "a" "${SERVER_PORT_A}" "${BUNDLE_PORT_A}"
  write_test_server_config "b" "${SERVER_PORT_B}" "${BUNDLE_PORT_B}"
  start_test_servers
}

setup_test_agents() {
  log "=== Setting up Test Agents (domain-a.test, domain-b.test) ==="
  write_test_agent_config  "a" "${SERVER_PORT_A}"
  write_test_agent_config  "b" "${SERVER_PORT_B}"
  start_test_agents
}

# =============================================================================
# SUMMARY
# =============================================================================

print_summary() {
  echo ""
  echo "======================================================================="
  echo "✅ SPIRE environment is up and running!"
  echo ""
  echo "  Admin (admin.app)"
  echo "    gRPC:            127.0.0.1:${ADMIN_SERVER_PORT}"
  echo "    Bundle endpoint: https://127.0.0.1:${ADMIN_BUNDLE_PORT}  (https_web)"
  echo "    Workload:        spiffe://admin.app/spire_admin"
  echo "    Logs:            ${ADMIN_DIR}/admin-server.log"
  echo "                     ${ADMIN_DIR}/admin-agent.log"
  echo ""
  echo "  Server A (domain-a.test)"
  echo "    gRPC:            127.0.0.1:${SERVER_PORT_A}"
  echo "    Bundle endpoint: https://127.0.0.1:${BUNDLE_PORT_A}  (https_web)"
  echo "    Workload:        spiffe://domain-a.test/workload-a"
  echo "    Logs:            ${TEST_DIR}/test-server-a.log"
  echo "                     ${TEST_DIR}/test-agent-a.log"
  echo ""
  echo "  Server B (domain-b.test)"
  echo "    gRPC:            127.0.0.1:${SERVER_PORT_B}"
  echo "    Bundle endpoint: https://127.0.0.1:${BUNDLE_PORT_B}  (https_web)"
  echo "    Workload:        spiffe://domain-b.test/workload-b"
  echo "    Logs:            ${TEST_DIR}/test-server-b.log"
  echo "                     ${TEST_DIR}/test-agent-b.log"
  echo ""
  echo "======================================================================="
}

# =============================================================================
# ENTRYPOINT
# =============================================================================

main() {
  check_deps

  trap "
    log 'Shutting down all SPIRE processes...'
    kill \$(jobs -p) 2>/dev/null || true
  " EXIT INT TERM

  cleanup_admin
  setup_admin_server
  cleanup_test
  setup_test_server
  generate_trust_bundle
  bootstrap_federation
  setup_admin_agent
  setup_test_agents
  print_summary
  wait
}

main "$@"
