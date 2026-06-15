#!/bin/bash

# This script generates a self-signed TLS certificate and private key
# for local development and testing of the spire-admin-web server.

CERT_DIR="certs"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"
CA_FILE="$CERT_DIR/rootCA.pem"
CA_KEY="$CERT_DIR/rootCA.key"

mkdir -p "$CERT_DIR"

echo "1. Generating Local Root CA..."
# Create the Root CA Key and Certificate
openssl genrsa -out "$CA_KEY" 4096
openssl req -x509 -new -nodes -key "$CA_KEY" -sha256 -days 1024 -out "$CA_FILE" -subj "/CN=MyLocalDevelopmentCA"

echo "2. Generating Server Certificate for localhost..."
# Create the Server Private Key
openssl genrsa -out "$KEY_FILE" 2048

# Create a Certificate Signing Request (CSR)
openssl req -new -key "$KEY_FILE" -out "$CERT_DIR/server.csr" -subj "/CN=localhost"

# Create an extension file to define Subject Alternative Names (SAN)
# Modern browsers require SAN to trust localhost certs
cat > "$CERT_DIR/server.ext" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF

# Sign the Server CSR with our Local Root CA
openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CA_FILE" -CAkey "$CA_KEY" \
-CAcreateserial -out "$CERT_FILE" -days 365 -sha256 -extfile "$CERT_DIR/server.ext"

if [ $? -eq 0 ]; then
    echo "------------------------------------------------"
    echo "SUCCESS: Local CA and Signed Certificate generated."
    echo "IMPORTANT: To remove browser warnings, you must import '$CA_FILE'"
    echo "into your browser/OS 'Trusted Root Certification Authorities' store."
    echo "------------------------------------------------"
else
    echo "Failed to generate certificates. Please ensure openssl is installed."
    exit 1
fi
