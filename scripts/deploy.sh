#!/usr/bin/env bash
# =============================================================================
# VoltIQ — Lambda Deploy Script
# Builds, zips, and uploads all three Lambda functions to AWS.
#
# Prerequisites:
#   - Go 1.21+ installed and on PATH
#   - AWS CLI v2 configured (aws configure)
#   - Lambda functions voltiq-simulator, voltiq-processor, voltiq-broadcaster
#     already exist in the target region (create them first in AWS Console)
#
# Usage:
#   chmod +x scripts/deploy.sh
#   bash scripts/deploy.sh
#
# To deploy a single function only:
#   FUNCTIONS="processor" bash scripts/deploy.sh
# =============================================================================

set -euo pipefail

REGION="${AWS_REGION:-af-south-1}"
FUNCTIONS="${FUNCTIONS:-simulator processor broadcaster}"

# Move to the repo root regardless of where the script is called from
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

echo "========================================"
echo "  VoltIQ Lambda Deploy"
echo "  Region : ${REGION}"
echo "  Targets: ${FUNCTIONS}"
echo "========================================"
echo ""

for fn in ${FUNCTIONS}; do
    LAMBDA_NAME="voltiq-${fn}"
    echo "──────────────────────────────────────"
    echo "  Building voltiq-${fn}..."
    echo "──────────────────────────────────────"

    # Cross-compile for AWS Lambda Linux ARM64 (Graviton2)
    # CGO_ENABLED=0 → fully static binary, no glibc dependency
    # -ldflags='-s -w' → strip debug symbols, reduces binary size ~30%
    # -o bootstrap → Lambda custom runtime requires this exact filename
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
        go build \
        -ldflags='-s -w' \
        -o bootstrap \
        "./cmd/${fn}/"

    echo "  ✓ Build complete"

    # Package the binary
    zip "${fn}.zip" bootstrap
    echo "  ✓ Zipped → ${fn}.zip"

    # Upload code to Lambda
    aws lambda update-function-code \
        --function-name "${LAMBDA_NAME}" \
        --zip-file "fileb://${fn}.zip" \
        --region "${REGION}" \
        --output text \
        --query 'FunctionArn'

    echo "  ✓ Uploaded to ${LAMBDA_NAME}"

    # Wait for update to complete before changing config
    aws lambda wait function-updated \
        --function-name "${LAMBDA_NAME}" \
        --region "${REGION}" 2>/dev/null || true

    # Switch runtime from Python placeholder → Go custom runtime
    aws lambda update-function-configuration \
        --function-name "${LAMBDA_NAME}" \
        --runtime provided.al2023 \
        --handler bootstrap \
        --region "${REGION}" \
        --output text \
        --query 'FunctionArn'

    echo "  ✓ Runtime switched to provided.al2023"

    # Clean up local build artifacts
    rm -f bootstrap "${fn}.zip"
    echo "  ✓ Cleaned up"
    echo ""
done

echo "========================================"
echo "  All functions deployed successfully!"
echo "  Next: run seed_stations.sh if you"
echo "  haven't seeded DynamoDB yet."
echo "========================================"
