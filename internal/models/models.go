// Package models defines all shared data structures used across VoltIQ Lambda functions.
// JSON tags are used for SQS/WebSocket serialisation.
// dynamodbav tags are used for DynamoDB marshalling via attributevalue.
package models

// LatLng represents a GPS coordinate pair.
type LatLng struct {
	Lat float64 `json:"lat" dynamodbav:"lat"`
	Lng float64 `json:"lng" dynamodbav:"lng"`
}

// TelemetryEvent is emitted by the simulator into the SQS queue
// once per vehicle per EventBridge tick.
type TelemetryEvent struct {
	VehicleID   string  `json:"vehicle_id"`
	Timestamp   string  `json:"timestamp"`
	BatteryPct  float64 `json:"battery_pct"`
	Location    LatLng  `json:"location"`
	NextTripAt  string  `json:"next_trip_at"` // "HH:MM" in WAT
	NextTripKM  float64 `json:"next_trip_km"`
	IsCharging  bool    `json:"is_charging"`
	OdometerKM  float64 `json:"odometer_km"`
}

// ChargingStation represents a physical EV charging point.
// Stored in DynamoDB table ChargingStations.
// DistanceKM is computed at runtime by the processor and never stored.
type ChargingStation struct {
	StationID      string  `json:"station_id"`
	Name           string  `json:"name"`
	Location       LatLng  `json:"location"`
	AvailablePorts int     `json:"available_ports"`
	MaxKW          float64 `json:"max_kw"`
	PricePerKWH    float64 `json:"price_per_kwh"`
	DistanceKM     float64 `json:"distance_km"     dynamodbav:"-"` // runtime only
}

// ChargingDecision is the AI output from Amazon Nova Lite via Bedrock.
// Stored as a nested Map inside VehicleState in DynamoDB.
type ChargingDecision struct {
	VehicleID     string  `json:"vehicle_id"      dynamodbav:"VehicleID"`
	Action        string  `json:"action"          dynamodbav:"Action"`        // CHARGE_NOW | CHARGE_LATER | SKIP
	StationID     string  `json:"station_id"      dynamodbav:"StationID"`
	ChargeStartAt string  `json:"charge_start_at" dynamodbav:"ChargeStartAt"`
	ChargeEndAt   string  `json:"charge_end_at"   dynamodbav:"ChargeEndAt"`
	EstCostNaira  float64 `json:"est_cost_naira"  dynamodbav:"EstCostNaira"`
	SavingsNaira  float64 `json:"savings_naira"   dynamodbav:"SavingsNaira"`
	Reasoning     string  `json:"reasoning"       dynamodbav:"Reasoning"`
	Confidence    float64 `json:"confidence"      dynamodbav:"Confidence"`
}

// VehicleState is the primary record written by the processor and read by
// the broadcaster. Stored in DynamoDB table VehicleState with PK VehicleID.
// DynamoDB Streams on this table trigger the broadcaster Lambda.
type VehicleState struct {
	VehicleID       string           `json:"vehicle_id"        dynamodbav:"VehicleID"`
	LastSeen        string           `json:"last_seen"         dynamodbav:"LastSeen"`
	BatteryPct      float64          `json:"battery_pct"       dynamodbav:"BatteryPct"`
	Location        LatLng           `json:"location"          dynamodbav:"Location"`
	IsCharging      bool             `json:"is_charging"       dynamodbav:"IsCharging"`
	LastDecision    ChargingDecision `json:"last_decision"     dynamodbav:"LastDecision"`
	TotalSavingsNGN float64          `json:"total_savings_ngn" dynamodbav:"TotalSavingsNGN"`
	UpdatedAt       string           `json:"updated_at"        dynamodbav:"UpdatedAt"`
}

// WebSocketMessage is the payload pushed to the browser dashboard
// via API Gateway WebSocket PostToConnection.
// Intentionally flat — no nested objects — for easy JS consumption.
type WebSocketMessage struct {
	Type       string  `json:"type"`        // Always "VEHICLE_UPDATE"
	VehicleID  string  `json:"vehicle_id"`
	BatteryPct float64 `json:"battery_pct"`
	IsCharging bool    `json:"is_charging"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Action     string  `json:"action"`
	StationID  string  `json:"station_id"`
	CostNaira  float64 `json:"cost_naira"`
	SavingsNGN float64 `json:"savings_ngn"`
	TotalSaved float64 `json:"total_saved"`
	Reasoning  string  `json:"reasoning"`
	UpdatedAt  string  `json:"updated_at"`
}

// Connection holds a single API Gateway WebSocket connection ID.
// Stored in DynamoDB table Connections with PK ConnectionID.
type Connection struct {
	ConnectionID string `dynamodbav:"ConnectionID"`
}
