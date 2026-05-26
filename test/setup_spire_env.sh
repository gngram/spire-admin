#!/usr/bin/bash
set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <data_directory_path>"
    exit 1
fi

DATA_DIR="$1"
# Ensure DATA_DIR is an absolute path
if [[ ! "$DATA_DIR" = /* ]]; then
  DATA_DIR="$(pwd)/$DATA_DIR"
fi

# Clean up previous runs
echo "Cleaning up any existing SPIRE processes and old data..."
killall spire-server spire-agent 2>/dev/null || true
rm -rf "${DATA_DIR}"
mkdir -p "${DATA_DIR}"

echo "Generating configurations..."

# ==========================================
# Server 1 Configuration (domain1.test)
# ==========================================
cat <<EOF > "${DATA_DIR}/server1.conf"
server {
    bind_address = "127.0.0.1"
    bind_port = 8081
    socket_path = "${DATA_DIR}/server1.sock"
    trust_domain = "domain1.test"
    admin_ids = ["spiffe://domain1.test/spidar-admin"] 
    data_dir = "${DATA_DIR}/server1_data"
    log_level = "INFO"
    federation {
        bundle_endpoint {
            address = "127.0.0.1"
            port = 8443
        }
        federates_with "domain2.test" {
            bundle_endpoint_url = "https://127.0.0.1:8444"
            bundle_endpoint_profile "https_spiffe" {
                endpoint_spiffe_id = "spiffe://domain2.test/spire/server"
            }
        }
    }
}
plugins {
    DataStore "sql" {
        plugin_data {
            database_type = "sqlite3"
            connection_string = "${DATA_DIR}/server1_data/datastore.sqlite3"
        }
    }
    KeyManager "disk" {
        plugin_data {
            keys_path = "${DATA_DIR}/server1_data/keys.json"
        }
    }
    NodeAttestor "join_token" {
        plugin_data {}
    }
}
EOF

# ==========================================
# Server 2 Configuration (domain2.test)
# ==========================================
cat <<EOF > "${DATA_DIR}/server2.conf"
server {
    bind_address = "127.0.0.1"
    bind_port = 8082
    socket_path = "${DATA_DIR}/server2.sock"
    trust_domain = "domain2.test"
    admin_ids = ["spiffe://domain1.test/spidar-admin"] 
    data_dir = "${DATA_DIR}/server2_data"
    log_level = "INFO"
    federation {
        bundle_endpoint {
            address = "127.0.0.1"
            port = 8444
        }
        federates_with "domain1.test" {
            bundle_endpoint_url = "https://127.0.0.1:8443"
            bundle_endpoint_profile "https_spiffe" {
                endpoint_spiffe_id = "spiffe://domain1.test/spire/server"
            }
        }
    }
}
plugins {
    DataStore "sql" {
        plugin_data {
            database_type = "sqlite3"
            connection_string = "${DATA_DIR}/server2_data/datastore.sqlite3"
        }
    }
    KeyManager "disk" {
        plugin_data {
            keys_path = "${DATA_DIR}/server2_data/keys.json"
        }
    }
    NodeAttestor "join_token" {
        plugin_data {}
    }
}
EOF

# ==========================================
# Agent Configurations (Agents 1-4)
# ==========================================
for i in 1 2 3 4; do
    PORT=$((i<=2 ? 8081 : 8082))
    DOMAIN=$((i<=2 ? 1 : 2))
    cat <<EOF > "${DATA_DIR}/agent$i.conf"
agent {
    data_dir = "${DATA_DIR}/agent${i}_data"
    log_level = "INFO"
    server_address = "127.0.0.1"
    server_port = ${PORT}
    socket_path = "${DATA_DIR}/agent${i}.sock"
    trust_domain = "domain${DOMAIN}.test"
    trust_bundle_path = "${DATA_DIR}/server${DOMAIN}.bundle"
}
plugins {
    NodeAttestor "join_token" {
        plugin_data {}
    }
    KeyManager "disk" {
        plugin_data {
            directory = "${DATA_DIR}/agent${i}_data"
        }
    }
    WorkloadAttestor "unix" {
        plugin_data {}
    }
}
EOF
done

echo "Starting SPIRE Servers..."
spire-server run -config "${DATA_DIR}/server1.conf" > "${DATA_DIR}/server1.log" 2>&1 &
PID_S1=$!
spire-server run -config "${DATA_DIR}/server2.conf" > "${DATA_DIR}/server2.log" 2>&1 &
PID_S2=$!

# Graceful shutdown on script exit
trap "echo -e '\nShutting down SPIRE topology...'; kill $PID_S1 $PID_S2 \$(jobs -p) 2>/dev/null || true" EXIT

# Give servers a moment to initialize
sleep 3

echo "Bootstrapping dynamic federation between domains..."
spire-server bundle show -socketPath "${DATA_DIR}/server1.sock" > "${DATA_DIR}/server1.bundle"
spire-server bundle show -socketPath "${DATA_DIR}/server2.sock" > "${DATA_DIR}/server2.bundle"

spire-server bundle set -socketPath "${DATA_DIR}/server1.sock" -id "spiffe://domain2.test" -format pem -path "${DATA_DIR}/server2.bundle"
spire-server bundle set -socketPath "${DATA_DIR}/server2.sock" -id "spiffe://domain1.test" -format pem -path "${DATA_DIR}/server1.bundle"

echo "Registering Workloads & Starting SPIRE Agents..."
# Setup Server 1 (Agents 1 & 2) and Server 2 (Agents 3 & 4)
for i in 1 2 3 4; do
    if [ "$i" -le 2 ]; then
        SOCK="${DATA_DIR}/server1.sock"
        DOMAIN="domain1.test"
        FED_DOMAIN="spiffe://domain2.test"
    else
        SOCK="${DATA_DIR}/server2.sock"
        DOMAIN="domain2.test"
        FED_DOMAIN="spiffe://domain1.test"
    fi
    
    TOKEN=$(spire-server token generate -socketPath "$SOCK" -spiffeID "spiffe://$DOMAIN/agent$i" | awk '{print $2}')
    
    # Register standard workload
    spire-server entry create -socketPath "$SOCK" -parentID "spiffe://$DOMAIN/agent$i" \
        -spiffeID "spiffe://$DOMAIN/workload$i" -selector unix:uid:$(id -u) -federatesWith "$FED_DOMAIN" > /dev/null

    # Start Agent
    spire-agent run -config "${DATA_DIR}/agent$i.conf" -joinToken "$TOKEN" > "${DATA_DIR}/agent$i.log" 2>&1 &
done

echo "Registering 'spidar-admin' with -admin flag on both servers..."
spire-server entry create -socketPath "${DATA_DIR}/server1.sock" -parentID spiffe://domain1.test/agent1 \
    -spiffeID spiffe://domain1.test/spidar-admin -selector unix:uid:$(id -u) -admin -federatesWith spiffe://domain2.test > /dev/null

#spire-server entry create -socketPath "${DATA_DIR}/server2.sock" -parentID spiffe://domain2.test/agent3 \
#    -spiffeID spiffe://domain2.test/spidar-admin -selector unix:uid:$(id -u) -admin -federatesWith spiffe://domain1.test > /dev/null

echo ""
echo "======================================================================="
echo "✅ SPIRE Environment is up and running!"
echo "   - Server 1: 127.0.0.1:8081 (domain1.test)     Socket: ${DATA_DIR}/server1.sock"
echo "   - Server 2: 127.0.0.1:8082 (domain2.test)     Socket: ${DATA_DIR}/server2.sock"
echo "   - Agents: 4 (2 per server, Status: Running)"
echo "     - Agent 1 Socket: ${DATA_DIR}/agent1.sock"
echo "     - Agent 2 Socket: ${DATA_DIR}/agent2.sock"
echo "     - Agent 3 Socket: ${DATA_DIR}/agent3.sock"
echo "     - Agent 4 Socket: ${DATA_DIR}/agent4.sock"
echo "   - Workloads: 4 (1 per agent)"
echo "   - Federation: domain1.test <---> domain2.test"
echo "   - Admin Workloads: spiffe://domain{1,2}.test/spidar-admin"
echo ""
echo "Logs and configuration files are located in: ${DATA_DIR}/"
echo "Press [CTRL+C] to stop the test servers and clean up processes."
echo "======================================================================="
wait
