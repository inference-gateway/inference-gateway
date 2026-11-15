#!/usr/bin/env sh
# Issues a throwaway CA, one server certificate and one client certificate for
# the mutual TLS examples. Example-only: the keys are world-readable so the
# gateway container or pod can read them whatever user it runs as.
set -eu

DIR="${1:-$(dirname "$0")/certs}"

if [ -f "$DIR/client.key" ]; then
  echo "certificates already present in $DIR"
  exit 0
fi

mkdir -p "$DIR"

openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -subj "/CN=inference-gateway-example-ca" \
  -keyout "$DIR/ca.key" -out "$DIR/ca.crt" 2>/dev/null

issue() {
  name="$1"
  usage="$2"
  san="$3"

  openssl req -newkey rsa:2048 -nodes -subj "/CN=$name" \
    -keyout "$DIR/$name.key" -out "$DIR/$name.csr" 2>/dev/null
  printf 'extendedKeyUsage=%s\nsubjectAltName=%s\n' "$usage" "$san" >"$DIR/$name.ext"
  openssl x509 -req -in "$DIR/$name.csr" -days 365 \
    -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" -CAcreateserial \
    -extfile "$DIR/$name.ext" -out "$DIR/$name.crt" 2>/dev/null
  rm -f "$DIR/$name.csr" "$DIR/$name.ext"
}

# One server certificate for both TLS servers in the example: the gateway and
# the provider in front of it.
issue server serverAuth 'DNS:localhost,DNS:inference-gateway,DNS:mock-provider,IP:127.0.0.1'
issue client clientAuth 'DNS:inference-gateway-client'

chmod 644 "$DIR"/*.key

echo "certificates written to $DIR"
