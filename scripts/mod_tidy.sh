#!/bin/bash
export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"
export PATH=$PATH:/usr/local/go/bin
cd /mnt/c/Users/USER/Desktop/VoltIQ

echo "--- go mod tidy (removing stale Kinesis) ---"
go mod tidy
echo "Done — go.mod cleaned"
