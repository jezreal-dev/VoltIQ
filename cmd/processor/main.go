// Lambda #2 — voltiq-processor
//
// Triggered by SQS queue "voltiq-telemetry".
// For each TelemetryEvent message it:
//  1. Decodes the event from the SQS message body (JSON string)
//  2. Gets the current grid electricity rate (Lagos WAT tariff tiers)
//  3. Loads all charging stations from DynamoDB, sorts by distance, picks nearest 2
//  4. Reads previous VehicleState to carry forward cumulative savings
//  5. Builds a Bedrock prompt and calls Claude Haiku for a ChargingDecision
//  6. Accumulates savings into TotalSavingsNGN
//  7. Writes updated VehicleState to DynamoDB (triggers broadcaster via DynamoDB Streams)
//  8. Archives raw message body to S3
//
// Env vars:
//
//	DYNAMO_REGION  — AWS region for DynamoDB (required)
//	BEDROCK_REGION — AWS region for Bedrock  (required, must have Haiku access)
//	S3_BUCKET      — S3 bucket name for telemetry archive (required)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/voltiq/voltiq/internal/bedrock"
	"github.com/voltiq/voltiq/internal/dynamo"
	"github.com/voltiq/voltiq/internal/models"
	"github.com/voltiq/voltiq/internal/pricing"
)

var (
	dynamoClient  *dynamo.Client
	bedrockClient *bedrock.BedrockClient
	s3Client      *s3.Client
	s3Bucket      string
)

func init() {
	dynamoRegion := os.Getenv("DYNAMO_REGION")
	if dynamoRegion == "" {
		dynamoRegion = "af-south-1"
	}
	bedrockRegion := os.Getenv("BEDROCK_REGION")
	if bedrockRegion == "" {
		bedrockRegion = "us-east-1" // Bedrock not available in af-south-1
	}
	s3Bucket = os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "voltiq-telemetry-archive-376791751274"
	}

	var err error
	dynamoClient, err = dynamo.NewClient(dynamoRegion)
	if err != nil {
		log.Fatalf("processor: failed to create DynamoDB client: %v", err)
	}

	bedrockClient, err = bedrock.NewBedrockClient(bedrockRegion)
	if err != nil {
		log.Fatalf("processor: failed to create Bedrock client: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(dynamoRegion),
	)
	if err != nil {
		log.Fatalf("processor: failed to load AWS config: %v", err)
	}
	s3Client = s3.NewFromConfig(cfg)
}

// handler processes all SQS messages in a batch.
// Errors per-record are logged but do not abort the batch.
func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	for _, record := range sqsEvent.Records {
		if err := processRecord(ctx, record); err != nil {
			log.Printf("processor: message %s error: %v", record.MessageId, err)
			// Continue to next record — partial failure is acceptable
		}
	}
	return nil
}

// processRecord executes the full pipeline for a single SQS message.
func processRecord(ctx context.Context, record events.SQSMessage) error {
	// 1. Decode TelemetryEvent from SQS message body (JSON string)
	rawData := []byte(record.Body)
	var event models.TelemetryEvent
	if err := json.Unmarshal(rawData, &event); err != nil {
		return fmt.Errorf("unmarshal telemetry: %w", err)
	}
	log.Printf("processor: processing %s battery=%.1f%%", event.VehicleID, event.BatteryPct)

	// 2. Get current grid rate (Lagos WAT tariff)
	gridRate := pricing.GetCurrentRateNGN(time.Now())

	// 3. Load stations, compute distances, sort, take nearest 2
	stations, err := dynamoClient.GetAllStations()
	if err != nil {
		return fmt.Errorf("get stations: %w", err)
	}
	for i := range stations {
		stations[i].DistanceKM = euclideanKM(event.Location, stations[i].Location)
	}
	sort.Slice(stations, func(i, j int) bool {
		return stations[i].DistanceKM < stations[j].DistanceKM
	})
	if len(stations) > 2 {
		stations = stations[:2]
	}

	// 4. Load previous VehicleState (for cumulative savings)
	existing, err := dynamoClient.GetVehicleState(event.VehicleID)
	if err != nil {
		log.Printf("processor: GetVehicleState %s failed (continuing): %v", event.VehicleID, err)
	}
	cumulativeSavings := 0.0
	if existing != nil {
		cumulativeSavings = existing.TotalSavingsNGN
	}

	// 5. Build Bedrock prompt
	prompt := buildPrompt(event, gridRate, stations)

	// 6. Call Bedrock → parse ChargingDecision
	rawDecision, err := bedrockClient.Invoke(prompt)
	if err != nil {
		log.Printf("processor: Bedrock invoke for %s failed: %v", event.VehicleID, err)
		// Fallback decision: skip — pipeline must not crash
		rawDecision = `{"action":"SKIP","reasoning":"Bedrock unavailable","confidence":0}`
	}
	decision := bedrockClient.ParseDecision(rawDecision, event.VehicleID)

	// 7. Compute deterministic savings — never trust the AI's ₦ numbers.
	// Savings = kWh needed × tariff differential vs worst-case rate.
	// This gives realistic, consistent figures the dashboard can proudly display.
	const (
		offPeakRate  = 185.0 // cheapest window (23:00–06:00 WAT)
		peakRate     = 320.0 // most expensive (18:00–23:00 WAT)
		batteryKWH   = 60.0  // fleet vehicle capacity in kWh
		chargeTarget = 80.0  // charge to 80% (industry standard)
	)
	kwhNeeded := math.Max(0, chargeTarget-event.BatteryPct) / 100.0 * batteryKWH
	switch decision.Action {
	case "CHARGE_LATER":
		// Savings from deferring to off-peak vs paying current rate now
		decision.SavingsNaira = kwhNeeded * math.Max(0, gridRate-offPeakRate)
	case "CHARGE_NOW":
		// Savings from charging at current (favourable) rate vs worst-case peak
		decision.SavingsNaira = kwhNeeded * math.Max(0, peakRate-gridRate)
	default:
		decision.SavingsNaira = 0
	}
	// Fill est_cost_naira if the AI left it empty
	if decision.EstCostNaira == 0 && kwhNeeded > 0 {
		decision.EstCostNaira = kwhNeeded * gridRate
	}

	// 8. Accumulate savings
	cumulativeSavings += decision.SavingsNaira

	// 9. Write updated VehicleState to DynamoDB
	// DynamoDB Streams on this table will fire the broadcaster Lambda.
	state := models.VehicleState{
		VehicleID:       event.VehicleID,
		LastSeen:        event.Timestamp,
		BatteryPct:      event.BatteryPct,
		Location:        event.Location,
		IsCharging:      event.IsCharging,
		LastDecision:    decision,
		TotalSavingsNGN: cumulativeSavings,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := dynamoClient.PutVehicleState(state); err != nil {
		return fmt.Errorf("put vehicle state: %w", err)
	}

	// 10. Archive raw event bytes to S3
	archiveKey := fmt.Sprintf("%s/%s.json", event.VehicleID, event.Timestamp)
	_, s3Err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(archiveKey),
		Body:   bytes.NewReader(rawData),
	})
	if s3Err != nil {
		// Non-fatal — log and continue
		log.Printf("processor: S3 archive for %s failed: %v", event.VehicleID, s3Err)
	}

	log.Printf("processor: %s → action=%s savings=₦%.2f total=₦%.2f",
		event.VehicleID, decision.Action, decision.SavingsNaira, cumulativeSavings)
	return nil
}

// euclideanKM returns an approximate distance in kilometres between two GPS points.
func euclideanKM(a, b models.LatLng) float64 {
	dlat := (a.Lat - b.Lat) * 111.0
	dlng := (a.Lng - b.Lng) * 111.0
	return math.Sqrt(dlat*dlat + dlng*dlng)
}

// buildPrompt constructs the user message sent to Claude Haiku.
func buildPrompt(event models.TelemetryEvent, gridRate float64, stations []models.ChargingStation) string {
	ratePeriod := "shoulder"
	switch gridRate {
	case 185.0:
		ratePeriod = "off-peak"
	case 320.0:
		ratePeriod = "peak"
	}

	chargingStr := "false"
	if event.IsCharging {
		chargingStr = "true"
	}

	// Current time in WAT (UTC+1) so Claude can reason about tariff windows
	wat := time.FixedZone("WAT", 1*60*60)
	nowWAT := time.Now().In(wat).Format("15:04 WAT")

	// Explicit urgency flag — Claude must not miss this
	urgency := ""
	if event.BatteryPct < 20 {
		urgency = fmt.Sprintf("\n⚠️  CRITICAL: Battery is %.1f%% — below 20%%. You MUST return action=CHARGE_NOW.\n", event.BatteryPct)
	} else if event.BatteryPct < 50 {
		urgency = fmt.Sprintf("\n⚠️  LOW: Battery is %.1f%% — below 50%%. Consider CHARGE_LATER to use off-peak rates.\n", event.BatteryPct)
	}

	stationsBlock := ""
	for i, s := range stations {
		stationsBlock += fmt.Sprintf("%d. %s (%s) - %.0fkW, %d ports, %.0f NGN/kWh, %.2fkm away\n",
			i+1, s.Name, s.StationID, s.MaxKW, s.AvailablePorts, s.PricePerKWH, s.DistanceKM)
	}

	return fmt.Sprintf(`Vehicle ID: %s
Battery: %.1f%%%s
Current time: %s
Next trip: %s, distance: %.1fkm
Currently charging: %s
Current electricity rate: %.2f NGN/kWh (%s)

Nearest charging stations:
%s
Respond with JSON only:
{
  "action": "CHARGE_NOW|CHARGE_LATER|SKIP",
  "station_id": "",
  "charge_start_at": "HH:MM",
  "charge_end_at": "HH:MM",
  "est_cost_naira": 0.0,
  "savings_naira": 0.0,
  "reasoning": "",
  "confidence": 0.0
}`,
		event.VehicleID,
		event.BatteryPct,
		urgency,
		nowWAT,
		event.NextTripAt,
		event.NextTripKM,
		chargingStr,
		gridRate,
		ratePeriod,
		stationsBlock,
	)
}


func main() {
	lambda.Start(handler)
}
