#!/bin/bash
# =============================================================
# VoltIQ WSL Deploy Script
# Builds all 3 Go Lambda binaries on Linux and deploys to AWS
#
# Usage (run from PowerShell):
#   wsl bash /mnt/c/Users/USER/Desktop/VoltIQ/scripts/wsl_deploy.sh
# =============================================================
set -euo pipefail

PROJECT_DIR="/mnt/c/Users/USER/Desktop/VoltIQ"
GO_VERSION="1.21.13"
REGION="af-south-1"

echo "=========================================="
echo "  VoltIQ WSL Build & Deploy"
echo "=========================================="

# ── 1. Ensure Go 1.21+ is available ────────────────────────
if command -v go &>/dev/null && go version 2>/dev/null | grep -qE "go1\.(2[1-9]|[3-9][0-9])"; then
    echo "✓ Go found: $(go version)"
else
    echo "Installing Go ${GO_VERSION}..."
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then DL_ARCH="amd64"; else DL_ARCH="arm64"; fi
    curl -sL "https://go.dev/dl/go${GO_VERSION}.linux-${DL_ARCH}.tar.gz" | sudo tar -C /usr/local -xz
    export PATH="$PATH:/usr/local/go/bin"
    echo "✓ Go installed: $(go version)"
fi
export PATH="$PATH:/usr/local/go/bin"

# ── 2. Ensure zip is available ─────────────────────────────
if ! command -v zip &>/dev/null; then
    echo "Installing zip..."
    sudo apt-get update -q
    sudo apt-get install -y -q zip
fi

# ── 3. Install AWS CLI in WSL if not present ───────────────
if ! command -v aws &>/dev/null; then
    echo "Installing AWS CLI v2 in WSL..."
    curl -sL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
    unzip -q /tmp/awscliv2.zip -d /tmp/awscliv2
    sudo /tmp/awscliv2/aws/install --update
    rm -rf /tmp/awscliv2.zip /tmp/awscliv2
    echo "✓ AWS CLI installed: $(aws --version)"
else
    echo "✓ AWS CLI found: $(aws --version)"
fi

# ── 4. Point AWS CLI at Windows credentials ────────────────
# Windows .aws files are accessible via /mnt/c in WSL2
export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"

echo ""
echo "Verifying AWS credentials..."
aws sts get-caller-identity --region "$REGION"
echo ""

# ── 4. Build & deploy ──────────────────────────────────────
cd "$PROJECT_DIR"
bash scripts/deploy.sh

echo ""
echo "=========================================="
echo "  All done! Lambda functions are live."
echo "  WebSocket: wss://1zc6ie2yp3.execute-api.af-south-1.amazonaws.com/prod"
echo "=========================================="
