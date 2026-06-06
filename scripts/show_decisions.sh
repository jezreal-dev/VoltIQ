#!/bin/bash
export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"

echo "=== Nova Lite AI Decisions + Reasoning ==="
aws dynamodb scan --region af-south-1 --table-name VehicleState \
  --query 'Items[*].{V:VehicleID.S, Action:LastDecision.M.Action.S, Station:LastDecision.M.StationID.S, Reasoning:LastDecision.M.Reasoning.S, Cost:LastDecision.M.EstCostNaira.N, Savings:LastDecision.M.SavingsNaira.N}' \
  --output json 2>&1 | python3 -c "
import json, sys
data = json.load(sys.stdin)
for v in sorted(data, key=lambda x: x.get('V','')):
    print()
    print('=' * 65)
    print(f\"Vehicle : {v.get('V','?')}  |  Action: {v.get('Action','?')}\")
    print(f\"Station : {v.get('Station','—')}\")
    print(f\"Est Cost: NGN {v.get('Cost','0')}  |  Savings: NGN {v.get('Savings','0')}\")
    r = v.get('Reasoning') or '(no reasoning returned)'
    print(f\"Reasoning: {r}\")
print()
print('=' * 65)
"
