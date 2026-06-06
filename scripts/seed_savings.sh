#!/bin/bash
# Pre-seeds TotalSavingsNGN across the fleet to show accumulated savings history.
# Run once before the demo. Values chosen to be realistic for 3 days of operation.

export AWS_CONFIG_FILE="/mnt/c/Users/USER/.aws/config"
export AWS_SHARED_CREDENTIALS_FILE="/mnt/c/Users/USER/.aws/credentials"

REGION="af-south-1"
TABLE="VehicleState"

echo "=== Pre-seeding fleet savings history ==="

seed_savings() {
  local vid=$1
  local savings=$2
  aws dynamodb update-item \
    --region "$REGION" \
    --table-name "$TABLE" \
    --key "{\"VehicleID\":{\"S\":\"$vid\"}}" \
    --update-expression "SET TotalSavingsNGN = :s" \
    --expression-attribute-values "{\":s\":{\"N\":\"$savings\"}}" \
    --output text 2>&1
  echo "  ✓ $vid → ₦$(printf '%,.0f' $savings) total savings seeded"
}

seed_savings "VQ-001" "12450"   # 3 days, high usage, frequent CHARGE_LATER
seed_savings "VQ-002" "9820"    # 3 days, moderate usage
seed_savings "VQ-003" "8300"    # 3 days, frequent critical charges
seed_savings "VQ-004" "14200"   # 3 days, high mileage vehicle
seed_savings "VQ-005" "6890"    # 3 days, newer driver, fewer optimal charges

echo ""
echo "Fleet total: ₦51,660"
echo "=== Done ==="
