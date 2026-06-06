#!/bin/bash
export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"
export PATH=$PATH:/usr/local/go/bin
cd /mnt/c/Users/USER/Desktop/VoltIQ

echo "==> Building voltiq-processor (Claude-3-Haiku fix)..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags='-s -w' -o bootstrap ./cmd/processor/
echo "  ✓ Build complete"
zip processor.zip bootstrap
echo "  ✓ Zipped"

aws lambda update-function-code \
  --function-name voltiq-processor \
  --zip-file fileb://processor.zip \
  --region af-south-1 \
  --query 'FunctionArn' --output text

echo "  ✓ Uploaded — waiting for update..."
aws lambda wait function-updated --function-name voltiq-processor --region af-south-1 2>/dev/null || true

aws lambda update-function-configuration \
  --function-name voltiq-processor \
  --runtime provided.al2023 \
  --handler bootstrap \
  --region af-south-1 \
  --query 'FunctionArn' --output text

rm -f bootstrap processor.zip
echo "  ✓ Done — voltiq-processor is live with correct model ID!"
