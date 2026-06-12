// Package main implements the voltiq-broadcaster Lambda function.
// It is triggered by DynamoDB Streams on the VehicleState table to push updated
// fleet states to all active WebSocket connections.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/voltiq/voltiq/internal/dynamo"
	"github.com/voltiq/voltiq/internal/models"
)

var (
	dynamoClient *dynamo.Client
	apigwClient  *apigatewaymanagementapi.Client
)

func init() {
	dynamoRegion := os.Getenv("DYNAMO_REGION")
	if dynamoRegion == "" {
		dynamoRegion = "af-south-1"
	}
	endpoint := os.Getenv("APIGW_ENDPOINT")

	var err error
	dynamoClient, err = dynamo.NewClient(dynamoRegion)
	if err != nil {
		log.Fatalf("broadcaster: failed to create DynamoDB client: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(dynamoRegion),
	)
	if err != nil {
		log.Fatalf("broadcaster: failed to load AWS config: %v", err)
	}

	// Override the endpoint so PostToConnection reaches the correct API Gateway stage.
	apigwClient = apigatewaymanagementapi.NewFromConfig(cfg,
		func(o *apigatewaymanagementapi.Options) {
			if endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
		},
	)
}

// handler processes all DynamoDB stream records in the batch.
func handler(ctx context.Context, dbEvent events.DynamoDBEvent) error {
	for _, record := range dbEvent.Records {
		// Only act on new or updated VehicleState items
		if record.EventName != "INSERT" && record.EventName != "MODIFY" {
			continue
		}
		if err := broadcastRecord(ctx, record); err != nil {
			log.Printf("broadcaster: record %s error: %v", record.EventID, err)
			// Continue — partial failure must not abort the batch
		}
	}
	return nil
}

// broadcastRecord extracts the new VehicleState and pushes it to all WebSocket clients.
func broadcastRecord(ctx context.Context, record events.DynamoDBEventRecord) error {
	newImage := record.Change.NewImage
	if newImage == nil {
		return nil
	}

	// Parse the DynamoDB stream image into a VehicleState
	state := stateFromImage(newImage)
	log.Printf("broadcaster: pushing %s battery=%.1f%% action=%s",
		state.VehicleID, state.BatteryPct, state.LastDecision.Action)

	// Build the WebSocket payload
	msg := models.WebSocketMessage{
		Type:       "VEHICLE_UPDATE",
		VehicleID:  state.VehicleID,
		BatteryPct: state.BatteryPct,
		IsCharging: state.IsCharging,
		Lat:        state.Location.Lat,
		Lng:        state.Location.Lng,
		Action:     state.LastDecision.Action,
		StationID:  state.LastDecision.StationID,
		CostNaira:  state.LastDecision.EstCostNaira,
		SavingsNGN: state.LastDecision.SavingsNaira,
		TotalSaved: state.TotalSavingsNGN,
		Reasoning:  state.LastDecision.Reasoning,
		UpdatedAt:  state.UpdatedAt,
	}
	if msg.UpdatedAt == "" {
		msg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Scan all active WebSocket connections
	connections, err := dynamoClient.ScanConnections()
	if err != nil {
		return err
	}
	if len(connections) == 0 {
		log.Printf("broadcaster: no active connections — skipping push for %s", state.VehicleID)
		return nil
	}

	// Push to every connected browser
	for _, conn := range connections {
		_, postErr := apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(conn.ConnectionID),
			Data:         msgBytes,
		})
		if postErr != nil {
			var gone *apigwtypes.GoneException
			if errors.As(postErr, &gone) {
				// Browser tab closed — remove the stale connection
				log.Printf("broadcaster: connection %s gone — deleting", conn.ConnectionID)
				if delErr := dynamoClient.DeleteConnection(conn.ConnectionID); delErr != nil {
					log.Printf("broadcaster: delete stale connection %s: %v", conn.ConnectionID, delErr)
				}
				continue
			}
			log.Printf("broadcaster: PostToConnection %s: %v", conn.ConnectionID, postErr)
		}
	}
	return nil
}

// stateFromImage manually parses a DynamoDB stream NewImage map into a VehicleState.
// The events.DynamoDBAttributeValue type is separate from the SDK types package,
// so we extract fields by name and type rather than using attributevalue.UnmarshalMap.
func stateFromImage(image map[string]events.DynamoDBAttributeValue) models.VehicleState {
	state := models.VehicleState{}

	if v, ok := image["VehicleID"]; ok {
		state.VehicleID = v.String()
	}
	if v, ok := image["LastSeen"]; ok {
		state.LastSeen = v.String()
	}
	if v, ok := image["BatteryPct"]; ok {
		state.BatteryPct = parseNumber(v.Number())
	}
	if v, ok := image["IsCharging"]; ok {
		state.IsCharging = v.Boolean()
	}
	if v, ok := image["TotalSavingsNGN"]; ok {
		state.TotalSavingsNGN = parseNumber(v.Number())
	}
	if v, ok := image["UpdatedAt"]; ok {
		state.UpdatedAt = v.String()
	}

	// Location is stored as a DynamoDB Map { "lat": N, "lng": N }
	if v, ok := image["Location"]; ok {
		loc := v.Map()
		if lat, ok := loc["lat"]; ok {
			state.Location.Lat = parseNumber(lat.Number())
		}
		if lng, ok := loc["lng"]; ok {
			state.Location.Lng = parseNumber(lng.Number())
		}
	}

	// LastDecision is stored as a DynamoDB Map with ChargingDecision fields
	if v, ok := image["LastDecision"]; ok {
		dec := v.Map()
		if a, ok := dec["Action"]; ok {
			state.LastDecision.Action = a.String()
		}
		if s, ok := dec["StationID"]; ok {
			state.LastDecision.StationID = s.String()
		}
		if c, ok := dec["ChargeStartAt"]; ok {
			state.LastDecision.ChargeStartAt = c.String()
		}
		if c, ok := dec["ChargeEndAt"]; ok {
			state.LastDecision.ChargeEndAt = c.String()
		}
		if c, ok := dec["EstCostNaira"]; ok {
			state.LastDecision.EstCostNaira = parseNumber(c.Number())
		}
		if s, ok := dec["SavingsNaira"]; ok {
			state.LastDecision.SavingsNaira = parseNumber(s.Number())
		}
		if r, ok := dec["Reasoning"]; ok {
			state.LastDecision.Reasoning = r.String()
		}
		if c, ok := dec["Confidence"]; ok {
			state.LastDecision.Confidence = parseNumber(c.Number())
		}
		if id, ok := dec["VehicleID"]; ok {
			state.LastDecision.VehicleID = id.String()
		}
	}

	return state
}

// parseNumber safely converts a DynamoDB Number string to float64.
func parseNumber(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func main() {
	lambda.Start(handler)
}
