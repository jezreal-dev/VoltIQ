# ⚡ VoltIQ

> **AI-Powered EV Fleet Charging Optimizer for Nigeria**
> Built on AWS · Go 1.21+ · Claude Haiku via Bedrock
> ONE WITH AI Hackathon 2026 · Arthurite Integrated · Lagos, Nigeria

---

## What Is VoltIQ?

VoltIQ is a real-time intelligence layer for electric vehicle fleet operators in Nigeria.
It continuously monitors your fleet's battery levels, predicts charging needs, and uses
AI (Claude Haiku on AWS Bedrock) to decide *when* and *where* each vehicle should charge —
automatically optimising for Nigeria's three-tier electricity tariff schedule.

Every decision is pushed live to a WebSocket dashboard so operators see the AI's reasoning
in real time, including the naira saved on every charge cycle.

---

## Architecture Overview

```
EventBridge (1 min)
      │
      ▼
┌─────────────────┐      Kinesis Stream       ┌─────────────────────┐
│  voltiq-         │ ──── voltiq-telemetry ──▶ │  voltiq-processor   │
│  simulator       │                           │                     │
│  (Lambda #1)     │                           │  1. Grid pricing    │
│                  │                           │  2. Station lookup  │
│  5 Lagos EVs     │                           │  3. Bedrock / AI    │
│  Battery drain   │                           │  4. Save state      │
│  GPS nudge       │                           │  5. Archive to S3   │
└─────────────────┘                           └──────────┬──────────┘
                                                         │
                                              DynamoDB Stream (VehicleState)
                                                         │
                                                         ▼
                                              ┌─────────────────────┐
                                              │  voltiq-broadcaster  │
                                              │  (Lambda #3)        │
                                              │                     │
                                              │  Scan Connections   │
                                              │  PostToConnection   │
                                              │  (WebSocket push)   │
                                              └──────────┬──────────┘
                                                         │
                                              API Gateway WebSocket
                                                         │
                                                         ▼
                                                   Dashboard / Browser
```

### AWS Services Used

| Service | Purpose |
|---|---|
| **Kinesis Data Stream** | Real-time telemetry ingestion |
| **Lambda (×3)** | Simulator · Processor · Broadcaster |
| **AWS Bedrock** | Claude Haiku AI charging decisions |
| **DynamoDB (×3)** | VehicleState · ChargingStations · Connections |
| **S3** | Raw telemetry archive |
| **API Gateway WebSocket** | Live dashboard push |
| **EventBridge** | 1-minute schedule triggers simulator |

---

## Folder Structure

```
voltiq/
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   ├── simulator/main.go       # Lambda #1 — telemetry emitter
│   ├── processor/main.go       # Lambda #2 — AI decision engine
│   └── broadcaster/main.go     # Lambda #3 — WebSocket pusher
├── internal/
│   ├── models/models.go        # Shared data structs
│   ├── bedrock/client.go       # Claude Haiku wrapper
│   ├── dynamo/client.go        # DynamoDB CRUD helpers
│   ├── kinesis/producer.go     # Kinesis PutRecord helper
│   └── pricing/grid.go         # Lagos electricity tariff tiers
└── scripts/
    ├── deploy.sh               # Build + zip + upload all 3 Lambdas
    └── seed_stations.sh        # Populate ChargingStations DynamoDB table
```

---

## Lagos Electricity Tariff Tiers

VoltIQ's pricing engine converts all times to **West Africa Time (WAT = UTC+1)** and applies:

| Period | Hours (WAT) | Rate (NGN/kWh) |
|---|---|---|
| Off-Peak | 23:00 – 05:59 | **₦185** |
| Shoulder | 06:00 – 17:59 | **₦225** |
| Peak | 18:00 – 22:59 | **₦320** |

The AI uses these rates to decide whether to charge now, charge later, or skip charging.

---

## Prerequisites

### Local Machine
- **Go 1.21+** — [download](https://go.dev/dl/)
- **AWS CLI v2** — [install guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- **bash** (Git Bash or WSL on Windows)

### AWS Account Setup
Before deploying, create these resources in region **`af-south-1`** (Cape Town):

#### DynamoDB Tables
```
Table: VehicleState
  Partition Key: VehicleID (String)
  Streams: Enabled → New image only

Table: ChargingStations
  Partition Key: StationID (String)

Table: Connections
  Partition Key: ConnectionID (String)
```

#### Kinesis Stream
```
Name: voltiq-telemetry
Shards: 1
```

#### S3 Bucket
```
Name: voltiq-telemetry-archive   (must be globally unique — add your account ID if needed)
Region: af-south-1
Access: Private
```

#### API Gateway
```
Type: WebSocket API
Name: voltiq-ws
Routes: $connect, $disconnect
Stage: prod
```
After deploying, note the **WebSocket endpoint URL** (format: `wss://xxxxxx.execute-api.af-south-1.amazonaws.com/prod`)

#### Lambda Functions (create empty, then deploy.sh fills them)
```
voltiq-simulator    → Runtime: provided.al2023, Arch: arm64
voltiq-processor    → Runtime: provided.al2023, Arch: arm64
voltiq-broadcaster  → Runtime: provided.al2023, Arch: arm64
```

#### Bedrock Model Access
In the AWS Console → Bedrock → Model Access (af-south-1 region):
Enable: **Anthropic Claude Haiku** (`anthropic.claude-haiku-20240307-v1:0`)

#### EventBridge Rule
```
Name: voltiq-simulator-schedule
Schedule: rate(1 minute)
Target: voltiq-simulator Lambda
```

---

## Environment Variables (per Lambda)

### voltiq-simulator
| Variable | Example Value |
|---|---|
| `KINESIS_STREAM_NAME` | `voltiq-telemetry` |

### voltiq-processor
| Variable | Example Value |
|---|---|
| `DYNAMO_REGION` | `af-south-1` |
| `S3_BUCKET` | `voltiq-telemetry-archive` |
| `BEDROCK_REGION` | `af-south-1` |

### voltiq-broadcaster
| Variable | Example Value |
|---|---|
| `DYNAMO_REGION` | `af-south-1` |
| `APIGW_ENDPOINT` | `https://xxxxxx.execute-api.af-south-1.amazonaws.com/prod` |

---

## Building Locally

```bash
# From the voltiq/ directory

# 1. Download all dependencies
go mod tidy

# 2. Verify everything compiles
go build ./...

# 3. Run static analysis
go vet ./...
```

Both commands must exit with zero errors before deploying.

---

## Deploying to AWS

```bash
# Configure AWS credentials first
aws configure

# Seed the charging stations into DynamoDB
chmod +x scripts/seed_stations.sh
bash scripts/seed_stations.sh

# Build and upload all 3 Lambda functions
chmod +x scripts/deploy.sh
bash scripts/deploy.sh
```

The `deploy.sh` script will:
1. Cross-compile each Lambda for `linux/arm64`
2. Zip the binary as `bootstrap` (AWS Lambda custom runtime convention)
3. Upload to the named Lambda function via AWS CLI
4. Clean up local build artifacts

---

## Running the Demo

After deployment, the pipeline starts automatically via EventBridge every 60 seconds.

**To trigger manually:**
```bash
# Trigger simulator
aws lambda invoke \
  --function-name voltiq-simulator \
  --region af-south-1 \
  --payload '{}' \
  /tmp/response.json && cat /tmp/response.json

# Check VehicleState table
aws dynamodb scan \
  --table-name VehicleState \
  --region af-south-1
```

**To watch WebSocket output:**
```bash
# Using wscat (npm install -g wscat)
wscat -c wss://YOUR_APIGW_ENDPOINT/prod
```
You will see `VEHICLE_UPDATE` JSON messages arriving every minute.

---

## Charging Stations (Seeded)

| Station ID | Name | Location | Ports | Max Power | Price |
|---|---|---|---|---|---|
| STN-LEKKI-A | Lekki Station Alpha | 6.4350°N, 3.4717°E | 4 | 50 kW | ₦225/kWh |
| STN-VICTORIA-A | Victoria Island Station | 6.4281°N, 3.4219°E | 6 | 100 kW | ₦240/kWh |
| STN-IKEJA-A | Ikeja Station Alpha | 6.6018°N, 3.3515°E | 3 | 50 kW | ₦220/kWh |
| STN-SURULERE-A | Surulere Station | 6.5005°N, 3.3567°E | 2 | 22 kW | ₦215/kWh |
| STN-AJAH-A | Ajah Station Alpha | 6.4698°N, 3.6034°E | 4 | 50 kW | ₦230/kWh |

---

## Vehicle Fleet (Simulated)

| Vehicle ID | Starting Location |
|---|---|
| VQ-001 | Lekki |
| VQ-002 | Victoria Island |
| VQ-003 | Ikeja |
| VQ-004 | Surulere |
| VQ-005 | Ajah |

---

## IAM Permissions Required

Each Lambda's execution role needs at minimum:

**voltiq-simulator**
```json
kinesis:PutRecord on arn:aws:kinesis:af-south-1:*:stream/voltiq-telemetry
```

**voltiq-processor**
```json
dynamodb:GetItem, PutItem, Scan  on VehicleState and ChargingStations tables
s3:PutObject                     on voltiq-telemetry-archive bucket
bedrock:InvokeModel              on claude-haiku model ARN
kinesis:GetRecords, GetShardIterator, DescribeStream, ListShards (Kinesis trigger)
```

**voltiq-broadcaster**
```json
dynamodb:Scan, DeleteItem        on Connections table
dynamodb:GetRecords (stream)     on VehicleState stream ARN
execute-api:ManageConnections    on your API Gateway ARN
```

---

## Demo Script — June 11, 2026

| Time | Action |
|---|---|
| 0:00 – 0:30 | Introduce VoltIQ — Nigeria's EV charging intelligence layer |
| 0:30 – 1:00 | Show live dashboard — 5 bus pins on Lagos map, battery cards |
| 1:00 – 1:45 | Focus on VQ-003 at 22% — watch AI reasoning appear live: `CHARGE_LATER` |
| 1:45 – 2:30 | Explain decision: Lekki Station Alpha at 23:00 off-peak, saving ₦1,400 |
| 2:30 – 3:00 | Show fleet-wide savings counter: **₦28,000+ saved today** |
| 3:00 – 3:30 | Highlight AWS architecture: Kinesis → Bedrock → DynamoDB → WebSocket |
| 3:30 – 4:00 | Close: "VoltIQ is the intelligence layer Nigeria's EV revolution is missing." |

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.21+ |
| AI Model | Anthropic Claude Haiku (via AWS Bedrock) |
| Streaming | AWS Kinesis Data Streams |
| Compute | AWS Lambda (arm64, custom runtime) |
| Database | AWS DynamoDB |
| Storage | AWS S3 |
| Real-time | AWS API Gateway WebSocket |
| Scheduling | AWS EventBridge |
| Region | af-south-1 (Cape Town) |

---

*Built by Team VoltIQ · Arthurite Integrated · Lagos, Nigeria*
*ONE WITH AI Hackathon 2026*
