// Package kinesis provides a Kinesis PutRecord helper for VoltIQ telemetry.
package kinesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/voltiq/voltiq/internal/models"
)

// Client is an interface over *kinesis.Client so this function
// can be unit-tested with a mock without hitting real AWS.
type Client interface {
	PutRecord(ctx context.Context, params *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error)
}

// PutTelemetryEvent serialises event to JSON and writes it to streamName.
// VehicleID is used as the Kinesis partition key, which guarantees ordering
// per vehicle across shards.
func PutTelemetryEvent(ctx context.Context, client Client, streamName string, event models.TelemetryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("kinesis: marshal event for %s: %w", event.VehicleID, err)
	}

	_, err = client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         data,
		PartitionKey: aws.String(event.VehicleID),
	})
	if err != nil {
		return fmt.Errorf("kinesis: PutRecord for %s: %w", event.VehicleID, err)
	}
	return nil
}
