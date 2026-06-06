#!/usr/bin/env bash
# =============================================================================
# VoltIQ — Seed Charging Stations
# Populates the DynamoDB ChargingStations table with 5 Lagos EV charging points.
#
# Run this ONCE before starting the demo.
# Safe to re-run — put-item with the same StationID will overwrite the record.
#
# Prerequisites:
#   - AWS CLI v2 configured (aws configure)
#   - DynamoDB table "ChargingStations" exists in the target region
#     with PK: StationID (String)
#
# Usage:
#   chmod +x scripts/seed_stations.sh
#   bash scripts/seed_stations.sh
# =============================================================================

set -euo pipefail

REGION="${AWS_REGION:-af-south-1}"
TABLE="ChargingStations"

echo "========================================"
echo "  VoltIQ — Seed Charging Stations"
echo "  Region : ${REGION}"
echo "  Table  : ${TABLE}"
echo "========================================"
echo ""

# ── STN-LEKKI-A ──────────────────────────────────────────────────────────────
echo "  Seeding STN-LEKKI-A (Lekki Station Alpha)..."
aws dynamodb put-item \
    --region "${REGION}" \
    --table-name "${TABLE}" \
    --item '{
        "StationID":      {"S": "STN-LEKKI-A"},
        "Name":           {"S": "Lekki Station Alpha"},
        "AvailablePorts": {"N": "4"},
        "MaxKW":          {"N": "50"},
        "PricePerKWH":    {"N": "225"},
        "Location": {"M": {
            "lat": {"N": "6.4350"},
            "lng": {"N": "3.4717"}
        }}
    }'
echo "  ✓ STN-LEKKI-A seeded"

# ── STN-VICTORIA-A ───────────────────────────────────────────────────────────
echo "  Seeding STN-VICTORIA-A (Victoria Island Station)..."
aws dynamodb put-item \
    --region "${REGION}" \
    --table-name "${TABLE}" \
    --item '{
        "StationID":      {"S": "STN-VICTORIA-A"},
        "Name":           {"S": "Victoria Island Station"},
        "AvailablePorts": {"N": "6"},
        "MaxKW":          {"N": "100"},
        "PricePerKWH":    {"N": "240"},
        "Location": {"M": {
            "lat": {"N": "6.4281"},
            "lng": {"N": "3.4219"}
        }}
    }'
echo "  ✓ STN-VICTORIA-A seeded"

# ── STN-IKEJA-A ──────────────────────────────────────────────────────────────
echo "  Seeding STN-IKEJA-A (Ikeja Station Alpha)..."
aws dynamodb put-item \
    --region "${REGION}" \
    --table-name "${TABLE}" \
    --item '{
        "StationID":      {"S": "STN-IKEJA-A"},
        "Name":           {"S": "Ikeja Station Alpha"},
        "AvailablePorts": {"N": "3"},
        "MaxKW":          {"N": "50"},
        "PricePerKWH":    {"N": "220"},
        "Location": {"M": {
            "lat": {"N": "6.6018"},
            "lng": {"N": "3.3515"}
        }}
    }'
echo "  ✓ STN-IKEJA-A seeded"

# ── STN-SURULERE-A ───────────────────────────────────────────────────────────
echo "  Seeding STN-SURULERE-A (Surulere Station)..."
aws dynamodb put-item \
    --region "${REGION}" \
    --table-name "${TABLE}" \
    --item '{
        "StationID":      {"S": "STN-SURULERE-A"},
        "Name":           {"S": "Surulere Station"},
        "AvailablePorts": {"N": "2"},
        "MaxKW":          {"N": "22"},
        "PricePerKWH":    {"N": "215"},
        "Location": {"M": {
            "lat": {"N": "6.5005"},
            "lng": {"N": "3.3567"}
        }}
    }'
echo "  ✓ STN-SURULERE-A seeded"

# ── STN-AJAH-A ───────────────────────────────────────────────────────────────
echo "  Seeding STN-AJAH-A (Ajah Station Alpha)..."
aws dynamodb put-item \
    --region "${REGION}" \
    --table-name "${TABLE}" \
    --item '{
        "StationID":      {"S": "STN-AJAH-A"},
        "Name":           {"S": "Ajah Station Alpha"},
        "AvailablePorts": {"N": "4"},
        "MaxKW":          {"N": "50"},
        "PricePerKWH":    {"N": "230"},
        "Location": {"M": {
            "lat": {"N": "6.4698"},
            "lng": {"N": "3.6034"}
        }}
    }'
echo "  ✓ STN-AJAH-A seeded"

echo ""
echo "========================================"
echo "  All 5 stations seeded successfully!"
echo ""
echo "  Verify with:"
echo "  aws dynamodb scan --table-name ChargingStations --region ${REGION}"
echo "========================================"
