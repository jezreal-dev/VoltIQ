#!/bin/bash
export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"

START=$(date -d '5 minutes ago' +%s%3N)

echo "=== voltiq-processor logs (last 5 min) ==="
aws logs filter-log-events \
  --region af-south-1 \
  --log-group-name /aws/lambda/voltiq-processor \
  --start-time "$START" \
  --query 'events[*].message' \
  --output text 2>&1 | head -100

echo ""
echo "=== Done ==="
