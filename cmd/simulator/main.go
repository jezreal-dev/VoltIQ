// Package main implements the voltiq-simulator Lambda function.
// It is triggered by EventBridge every 60 seconds to simulate a Lagos EV fleet
// of 5 vehicles, progressively draining battery, drifting GPS coordinates,
// and emitting telemetry events into SQS.
package main

import (
	"context"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/voltiq/voltiq/internal/dynamo"
	sqsproducer "github.com/voltiq/voltiq/internal/sqs"
	"github.com/voltiq/voltiq/internal/models"
)

// vehicleSeed holds the initial (fallback) state for each vehicle.
type vehicleSeed struct {
	id         string
	lat        float64
	lng        float64
	batteryPct float64
	odometerKM float64
}

// fleet is the hardcoded Lagos EV fleet.
// VQ-003 starts at 22% — the demo vehicle for the judge presentation.
var fleet = []vehicleSeed{
	{id: "VQ-001", lat: 6.4350, lng: 3.4717, batteryPct: 85.0, odometerKM: 12450.0},
	{id: "VQ-002", lat: 6.4281, lng: 3.4219, batteryPct: 67.0, odometerKM: 8920.0},
	{id: "VQ-003", lat: 6.6018, lng: 3.3515, batteryPct: 22.0, odometerKM: 19870.0},
	{id: "VQ-004", lat: 6.5005, lng: 3.3567, batteryPct: 92.0, odometerKM: 5340.0},
	{id: "VQ-005", lat: 6.4698, lng: 3.6034, batteryPct: 38.0, odometerKM: 23100.0},
}

var (
	sqsClient    *awssqs.Client
	dynamoClient *dynamo.Client
	sqsQueueURL  string
)

func init() {
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
	if sqsQueueURL == "" {
		sqsQueueURL = "https://sqs.af-south-1.amazonaws.com/376791751274/voltiq-telemetry"
	}

	dynamoRegion := os.Getenv("DYNAMO_REGION")
	if dynamoRegion == "" {
		dynamoRegion = "af-south-1"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(dynamoRegion),
	)
	if err != nil {
		log.Fatalf("simulator: failed to load AWS config: %v", err)
	}
	sqsClient = awssqs.NewFromConfig(cfg)

	dynamoClient, err = dynamo.NewClient(dynamoRegion)
	if err != nil {
		log.Fatalf("simulator: failed to create DynamoDB client: %v", err)
	}
}

// handler runs on every EventBridge invocation.
// It processes all 5 vehicles and emits their telemetry to SQS.
func handler(ctx context.Context) error {
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)

	for _, seed := range fleet {
		event, err := buildTelemetryEvent(seed, timestamp)
		if err != nil {
			log.Printf("simulator: build event for %s: %v", seed.id, err)
			continue
		}

		if err := sqsproducer.PutTelemetryEvent(ctx, sqsClient, sqsQueueURL, event); err != nil {
			log.Printf("simulator: emit %s: %v", seed.id, err)
			continue
		}

		log.Printf("simulator: emitted %s battery=%.1f%%", seed.id, event.BatteryPct)
	}
	return nil
}

// buildTelemetryEvent creates a TelemetryEvent for the given vehicle.
// It reads current battery and charging state from DynamoDB, then applies
// the battery lifecycle: drain while driving, recover while charging.
func buildTelemetryEvent(seed vehicleSeed, timestamp string) (models.TelemetryEvent, error) {
	battery := seed.batteryPct
	lat := seed.lat
	lng := seed.lng
	odometer := seed.odometerKM
	isCharging := false

	// Load persisted state from DynamoDB (battery, location, charging status)
	existing, err := dynamoClient.GetVehicleState(seed.id)
	if err != nil {
		log.Printf("simulator: DynamoDB read for %s failed (using defaults): %v", seed.id, err)
	}
	if existing != nil {
		battery = existing.BatteryPct
		lat = existing.Location.Lat
		lng = existing.Location.Lng
		isCharging = existing.IsCharging
	}

	// Battery lifecycle — prevents all vehicles piling up at the floor
	if isCharging {
		// Recovery mode: +2% per tick until 80% (simulates charging station)
		battery += 2.0
		if battery >= 80.0 {
			battery = 80.0
			isCharging = false // Charged enough — resume normal operation
		}
	} else {
		// Driving mode: drain 0.3–1.5% per tick (city driving + AC)
		drain := 0.3 + rand.Float64()*1.2
		battery -= drain
		if battery <= 10.0 {
			battery = 10.0
			isCharging = true // Trigger emergency charge at 10%
		}
	}

	// GPS: only drift when driving; clip to Lagos bounding box
	if !isCharging {
		lat += rand.Float64()*0.002 - 0.001 // ±0.001° ≈ ±110 m
		lng += rand.Float64()*0.002 - 0.001
	}
	// Lagos bounding box: 6.30–6.85°N, 3.10–3.80°E
	lat = math.Max(6.30, math.Min(6.85, lat))
	lng = math.Max(3.10, math.Min(3.80, lng))

	// Odometer only increases while driving
	if !isCharging {
		odometer += 1.0 + rand.Float64()*2.0
	}

	// Next trip time: dynamically 2 hours from now in WAT
	// (avoids AI receiving a trip time that has already passed)
	wat := time.FixedZone("WAT", 1*60*60)
	nextTrip := time.Now().In(wat).Add(2 * time.Hour)
	nextTripAt := nextTrip.Format("15:04")

	return models.TelemetryEvent{
		VehicleID:  seed.id,
		Timestamp:  timestamp,
		BatteryPct: battery,
		Location: models.LatLng{
			Lat: lat,
			Lng: lng,
		},
		NextTripAt: nextTripAt, // dynamic: current WAT + 2 hours
		NextTripKM: 45.0,       // typical Lagos cross-city route
		IsCharging: isCharging,
		OdometerKM: odometer,
	}, nil
}

func main() {
	lambda.Start(handler)
}
