# ⚡ VoltIQ

> **Real-Time AI-Powered EV Fleet Charging Optimization for Nigeria**
> Built on AWS Serverless · Go 1.21+ · Amazon Nova Lite via Bedrock
> Developed for the **ONE WITH AI Hackathon 2026** by Arthurite Integrated · Lagos, Nigeria

---

## 📋 What is VoltIQ?

VoltIQ is a real-time intelligence layer for electric vehicle (EV) fleet operators in Nigeria. It continuously monitors fleet telemetry (battery charge, state of health, GPS coordinates, next trip schedules) and uses Amazon Bedrock (Nova Lite) to make smart charging decisions. 

It optimizes for Nigeria's three-tier grid electricity tariff schedule, shifting heavy charging loads to off-peak periods (saving up to **₦5,184 per vehicle charge cycle**). Every decision is pushed in real-time to a live WebSocket dashboard.

---

## 🏗️ System Architecture

```
EventBridge (1 min schedule)
      │
      ▼
┌─────────────────┐         AWS SQS Queue        ┌─────────────────────┐
│  voltiq-         │ ─── [voltiq-telemetry] ───▶ │  voltiq-processor   │
│  simulator       │                             │                     │
│  (Lambda #1)     │                             │  1. Grid pricing    │
│                  │                             │  2. Station lookup  │
│  5 Lagos EVs     │                             │  3. Bedrock Nova AI │
│  Battery drain   │                             │  4. Save state      │
│  GPS nudge       │                             │  5. Archive to S3   │
└─────────────────┘                             └──────────┬──────────┘
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
                                                     React Dashboard
```

### AWS Services Infrastructure

| Service | Purpose |
|---|---|
| **Amazon SQS** | High-reliability, low-latency telemetry message queuing (decoupled from Kinesis limits) |
| **AWS Lambda (×3)** | Event-driven compute (Simulator, Processor, Broadcaster) compiled for `linux/arm64` |
| **Amazon Bedrock** | Native **Amazon Nova Lite** (`amazon.nova-lite-v1:0`) charging optimizer |
| **Amazon DynamoDB** | Persistent storage for `VehicleState`, `ChargingStations`, and active WebSocket `Connections` |
| **Amazon S3** | Raw telemetry storage archive for auditing and future model training |
| **Amazon API Gateway** | WebSocket endpoints for real-time live browser pushing |
| **Amazon EventBridge** | Cron scheduler triggering the simulator every 60 seconds |

---

## 📁 Repository Structure

```
voltiq/
├── go.mod
├── go.sum
├── README.md
├── walkthrough.md               # Deep dive architectural documentation
├── industry_features.md          # Real-world EV enterprise scaling roadmap
├── dashboard.html               # Local HTML test dashboard with WebSocket client
├── cmd/
│   ├── simulator/main.go        # Lambda #1: Telemetry emitter & battery drainage
│   ├── processor/main.go        # Lambda #2: AI decision engine & price calculator
│   └── broadcaster/main.go      # Lambda #3: DynamoDB Stream WebSocket broadcaster
├── internal/
│   ├── models/models.go         # Shared telemetry data structs & JSON contracts
│   ├── bedrock/client.go        # Amazon Bedrock Nova Lite runtime wrapper
│   ├── dynamo/client.go         # DynamoDB persistent storage helpers
│   ├── sqs/producer.go          # SQS SendMessage helper
│   └── pricing/grid.go          # WAT tariff pricing engine
└── scripts/
    ├── redeploy_all.sh          # One-click shell compilation and AWS deployer
    ├── seed_stations.sh         # Seeds the 5 charging stations in Lagos into DynamoDB
    └── seed_savings.sh          # Pre-seeds initial fleet savings history for demo showcase
```

---

## 🇳🇬 Lagos Grid Tariff Engine

VoltIQ's pricing calculations convert raw timestamps to **West Africa Time (WAT = UTC+1)** and evaluate grid rates dynamically:

| Tariff Period | Hours (WAT) | Rate (NGN/kWh) | Optimization Strategy |
|---|---|---|---|
| **Off-Peak** | 23:00 – 05:59 | **₦185** | Maximize charging in this window |
| **Shoulder** | 06:00 – 17:59 | **₦225** | Charge only if battery falls below critical limits |
| **Peak** | 18:00 – 22:59 | **₦320** | Avoid charging unless urgent to prevent grid overload |

---

## 🛠️ Deploying & Running the System

### Prerequisites
1. **Go 1.21+**
2. **AWS CLI v2** configured with `af-south-1` region credentials
3. **WSL / Bash** terminal access

### Local Build Validation
```bash
# Clean up dependencies
go mod tidy

# Build the workspace
go build ./...

# Run static checks
go vet ./...
```

### Cloud Deployment
```bash
# 1. Seed Lagos Charging Stations
chmod +x scripts/seed_stations.sh
./scripts/seed_stations.sh

# 2. Pre-seed Fleet Savings Data for Demo Showcase
chmod +x scripts/seed_savings.sh
./scripts/seed_savings.sh

# 3. Compile Go binaries & Deploy Lambdas
chmod +x scripts/redeploy_all.sh
./scripts/redeploy_all.sh
```

---

## 🛡️ IAM Permissions Matrix

| Lambda | Required Policies / Actions |
|---|---|
| **voltiq-simulator** | `sqs:SendMessage` on `voltiq-telemetry` |
| **voltiq-processor** | `dynamodb:GetItem`, `PutItem` on `VehicleState` & `ChargingStations`<br>`s3:PutObject` on telemetry bucket<br>`bedrock:InvokeModel` on `amazon.nova-lite-v1:0`<br>`sqs:ReceiveMessage`, `DeleteMessage` on queue |
| **voltiq-broadcaster** | `dynamodb:Scan`, `DeleteItem` on `Connections`<br>`dynamodb:GetRecords` on `VehicleState` DynamoDB Stream<br>`execute-api:ManageConnections` on WebSocket gateway |

---

## 🏆 ONE WITH AI Demo Script

| Time | Action | Presentation Focus |
|---|---|---|
| **0:00 – 0:45** | Project Pitch | Pitch VoltIQ as the financial margin protection layer for Lagos EV fleets. |
| **0:45 – 1:30** | Live Dashboard | Show the live React dashboard mapping 5 active e-buses navigating Lagos. |
| **1:30 – 2:15** | AI Ingest | Highlight vehicle **VQ-003** dropping to 18% battery. Watch the real-time AI decision card update to `CHARGE_LATER`. |
| **2:15 – 3:00** | Math Audit | Explain the AI reasoning: waiting until 23:00 Off-Peak saves ₦5,184 on this single charge. |
| **3:00 – 4:00** | Cloud Design | Showcase the serverless Go pipeline: SQS → Bedrock Nova Lite → DynamoDB Streams → WebSockets. |

---

*Developed by Team VoltIQ for the Arthurite Integrated ONE WITH AI Summit 2026.*
