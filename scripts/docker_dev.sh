#!/bin/sh
set -e

#
# Helper script for local development. Automatically builds and registers the
# plugin. Requires `vault` is installed and available on $PATH.
#

# Get the right dir
BACKEND_NAME="vmware-backend"

echo "==> Starting dev"

echo "--> Scratch dir"
echo "    Creating"
SCRATCH="/tmp"

echo "--> Vault server"
echo "    Writing config"
tee "$SCRATCH/vault.hcl" > /dev/null <<EOF
plugin_directory = "$SCRATCH/plugins"
EOF

echo "    Envvars"
export VAULT_DEV_ROOT_TOKEN_ID="root"
export VAULT_ADDR="http://127.0.0.1:8200"

echo "    Starting"
vault server \
  -dev \
  -log-level="debug" \
  -config="$SCRATCH/vault.hcl" \
  &
sleep 2
VAULT_PID=$!

function cleanup {
  echo ""
  echo "==> Cleaning up"
  kill -INT "$VAULT_PID"
  rm -rf "$SCRATCH"
}
trap cleanup EXIT

echo "    Authing"
vault login root &>/dev/null

echo "--> Building"
SHASUM=$(sha256sum "$SCRATCH/plugins/$BACKEND_NAME" | cut -d " " -f1)
echo "SHA sum: $SHASUM"

echo "    Registering plugin"
vault write sys/plugins/catalog/secret/${BACKEND_NAME} \
  sha_256="$SHASUM" \
  command="$BACKEND_NAME"

echo "    Mounting plugin"
vault secrets enable -path=vmware/1 -plugin-name=${BACKEND_NAME} plugin

echo "==> Ready!"
wait $!
