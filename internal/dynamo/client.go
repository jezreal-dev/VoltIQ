// Package dynamo provides DynamoDB CRUD helpers for VoltIQ tables.
package dynamo

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/voltiq/voltiq/internal/models"
)

const (
	tableVehicleState    = "VehicleState"
	tableChargingStations = "ChargingStations"
	tableConnections     = "Connections"
)

// Client wraps the AWS DynamoDB SDK client.
type Client struct {
	db *dynamodb.Client
}

// NewClient creates a DynamoDB Client using the given AWS region.
// Credentials are loaded from the Lambda execution role environment.
func NewClient(region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("dynamo: load config: %w", err)
	}
	return &Client{db: dynamodb.NewFromConfig(cfg)}, nil
}

// GetVehicleState retrieves a single VehicleState by primary key.
// Returns (nil, nil) when the item does not exist — callers must handle this case.
func (c *Client) GetVehicleState(vehicleID string) (*models.VehicleState, error) {
	result, err := c.db.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(tableVehicleState),
		Key: map[string]types.AttributeValue{
			"VehicleID": &types.AttributeValueMemberS{Value: vehicleID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamo: GetVehicleState %s: %w", vehicleID, err)
	}
	if result.Item == nil {
		return nil, nil // item not found — first-time vehicle
	}

	var state models.VehicleState
	if err := attributevalue.UnmarshalMap(result.Item, &state); err != nil {
		return nil, fmt.Errorf("dynamo: unmarshal VehicleState %s: %w", vehicleID, err)
	}
	return &state, nil
}

// PutVehicleState writes (upserts) a VehicleState record.
// Replaces the full item — partial updates are not used for simplicity.
func (c *Client) PutVehicleState(state models.VehicleState) error {
	item, err := attributevalue.MarshalMap(state)
	if err != nil {
		return fmt.Errorf("dynamo: marshal VehicleState %s: %w", state.VehicleID, err)
	}
	_, err = c.db.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(tableVehicleState),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamo: PutVehicleState %s: %w", state.VehicleID, err)
	}
	return nil
}

// GetAllStations returns all rows from the ChargingStations table.
// A full Scan is acceptable for the demo (≤5 stations). In production, use a GSI.
func (c *Client) GetAllStations() ([]models.ChargingStation, error) {
	result, err := c.db.Scan(context.Background(), &dynamodb.ScanInput{
		TableName: aws.String(tableChargingStations),
	})
	if err != nil {
		return nil, fmt.Errorf("dynamo: scan ChargingStations: %w", err)
	}

	var stations []models.ChargingStation
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &stations); err != nil {
		return nil, fmt.Errorf("dynamo: unmarshal stations: %w", err)
	}
	return stations, nil
}

// ScanConnections returns all active WebSocket connection IDs from the Connections table.
func (c *Client) ScanConnections() ([]models.Connection, error) {
	result, err := c.db.Scan(context.Background(), &dynamodb.ScanInput{
		TableName: aws.String(tableConnections),
	})
	if err != nil {
		return nil, fmt.Errorf("dynamo: scan Connections: %w", err)
	}

	var conns []models.Connection
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &conns); err != nil {
		return nil, fmt.Errorf("dynamo: unmarshal connections: %w", err)
	}
	return conns, nil
}

// DeleteConnection removes a stale WebSocket session from the Connections table.
// Called by the broadcaster when API Gateway returns HTTP 410 Gone.
func (c *Client) DeleteConnection(connectionID string) error {
	_, err := c.db.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tableConnections),
		Key: map[string]types.AttributeValue{
			"ConnectionID": &types.AttributeValueMemberS{Value: connectionID},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamo: DeleteConnection %s: %w", connectionID, err)
	}
	return nil
}
