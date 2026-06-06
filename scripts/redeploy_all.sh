#!/bin/bash
# Builds and deploys both voltiq-simulator and voltiq-processor in one shot.
set -e

export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"
export PATH=$PATH:/usr/local/go/bin
cd /mnt/c/Users/USER/Desktop/VoltIQ

echo "=============================================="
echo "  VoltIQ Full Redeploy — Simulator + Processor"
echo "=============================================="

build_and_deploy() {
  local name=$1
  local pkg=$2

  echo ""
  echo "── Building $name..."
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags='-s -w' -o bootstrap "./$pkg/"
  zip "${name}.zip" bootstrap
  echo "  ✓ Built and zipped"

  aws lambda update-function-code \
    --function-name "$name" \
    --zip-file "fileb://${name}.zip" \
    --region af-south-1 \
    --query 'FunctionArn' --output text

  echo "  ✓ Uploaded — waiting for update..."
  aws lambda wait function-updated --function-name "$name" --region af-south-1 2>/dev/null || true

  aws lambda update-function-configuration \
    --function-name "$name" \
    --runtime provided.al2023 \
    --handler bootstrap \
    --region af-south-1 \
    --query 'LastUpdateStatus' --output text

  rm -f bootstrap "${name}.zip"
  echo "  ✓ $name deployed!"
}

build_and_deploy "voltiq-simulator"  "cmd/simulator"
build_and_deploy "voltiq-processor"  "cmd/processor"

echo ""
echo "── Increasing processor Lambda timeout to 30s..."
aws lambda update-function-configuration \
  --function-name voltiq-processor \
  --timeout 30 \
  --region af-south-1 \
  --query 'Timeout' --output text
echo "  ✓ Timeout set to 30s"

echo ""
echo "=============================================="
echo "  All done! Both Lambdas are live."
echo "=============================================="
