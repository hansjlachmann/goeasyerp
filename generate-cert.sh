#!/bin/bash
# Generate a self-signed SSL certificate for internal HTTPS access.
# Usage: ./generate-cert.sh [IP_ADDRESS]
# The certificate is valid for 10 years.

set -e

CERT_DIR="./certs"
IP="${1:-$(hostname -I | awk '{print $1}')}"

mkdir -p "$CERT_DIR"

echo "Generating self-signed certificate for IP: $IP"

openssl req -x509 -nodes -days 3650 \
  -newkey rsa:2048 \
  -keyout "$CERT_DIR/selfsigned.key" \
  -out "$CERT_DIR/selfsigned.crt" \
  -subj "/C=NO/ST=Internal/L=Internal/O=GoEasyERP/CN=$IP" \
  -addext "subjectAltName=IP:$IP,IP:127.0.0.1"

echo "Certificate generated:"
echo "  $CERT_DIR/selfsigned.crt"
echo "  $CERT_DIR/selfsigned.key"
echo ""
echo "Valid for: 10 years"
echo "SAN: IP:$IP, IP:127.0.0.1"
