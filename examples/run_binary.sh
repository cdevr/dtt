#!/usr/bin/env bash
# Example: Running a simple shell script on a Proxmox VM
#
# Usage: ./run_binary.sh [BINARY_PATH]
#   BINARY_PATH: Optional path to the binary/script to run (overrides .env or default)
#
# Environment variables (can be set in .env):
#   PROXMOX_URL: Proxmox server URL (e.g., https://192.168.1.2:8006)
#   PROXMOX_NODE: Proxmox node name (default: pve)
#   TOKEN_ID: Proxmox API token ID (e.g., root@pam!tokenname)
#   TOKEN_SECRET: Proxmox API token secret
#   VM_ID: VM ID to use (default: 100)
#   VM_IP: VM IP address for SSH connection (required for upload/execute)
#   SSH_PASSWORD: SSH password for connecting to VM
#   BINARY_PATH: Path to binary to run (can be overridden by command line arg)

set -e

# Load environment variables from .env file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
ENV_FILE="$PROJECT_ROOT/.env"
DTT_BIN="$PROJECT_ROOT/dtt"

if [ ! -f "$ENV_FILE" ]; then
    echo "Error: .env file not found at $ENV_FILE"
    exit 1
fi

if [ ! -x "$DTT_BIN" ]; then
    echo "Error: dtt binary not found or not executable at $DTT_BIN"
    echo "Please build the project first"
    exit 1
fi

# Source the .env file
set -a
source "$ENV_FILE"
set +a

# Extract host from PROXMOX_URL (remove https:// and port)
PROXMOX_HOST=$(echo "$PROXMOX_URL" | sed -E 's|https?://||' | sed -E 's|:[0-9]+||')
PROXMOX_USER=$(echo "$TOKEN_ID" | cut -d'!' -f1)
PROXMOX_NODE="${PROXMOX_NODE:-pve}"

# Get binary path from command line argument, environment variable, or use default
if [ -n "$1" ]; then
    BINARY_PATH="$1"
else
    BINARY_PATH="${BINARY_PATH:-/path/to/your/script.sh}"
fi

# VM configuration - can be overridden by environment variables
VM_ID="${VM_ID:-100}"
VM_IP="${VM_IP:-}"
SSH_PASSWORD="${SSH_PASSWORD:-}"

# Make sure the binary exists and is executable
if [ ! -x "$BINARY_PATH" ]; then
    echo "Error: Binary not found or not executable: $BINARY_PATH"
    exit 1
fi

# Export token credentials to environment
export DTT_PROXMOX_TOKEN_ID="$TOKEN_ID"
export DTT_PROXMOX_TOKEN_SECRET="$TOKEN_SECRET"

# Export Proxmox SSH credentials if set
if [ -n "$PROXMOX_SSH_USER" ]; then
    export DTT_PROXMOX_SSH_USER="$PROXMOX_SSH_USER"
fi
if [ -n "$PROXMOX_SSH_PASSWORD" ]; then
    export DTT_PROXMOX_SSH_PASSWORD="$PROXMOX_SSH_PASSWORD"
fi

# Export VM SSH password if set
if [ -n "$SSH_PASSWORD" ]; then
    export DTT_SSH_PASSWORD="$SSH_PASSWORD"
fi

# Build the command
DTT_CMD=("$DTT_BIN" run "$BINARY_PATH" "$VM_ID" \
    --proxmox-host "$PROXMOX_HOST" \
    --proxmox-user "$PROXMOX_USER" \
    --proxmox-node "$PROXMOX_NODE" \
    --proxmox-insecure \
    --hostname "dtt-runner-${VM_ID}" \
    --image debian-11 \
    --memory 2048 \
    --cpu 2 \
    --cores 1 \
    --username dtt)

# Add VM IP if provided
if [ -n "$VM_IP" ]; then
    DTT_CMD+=(--vm-ip "$VM_IP")
fi

# Run the binary on a VM
"${DTT_CMD[@]}"

echo "Command completed for VM $VM_ID"
