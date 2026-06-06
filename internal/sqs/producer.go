// Package sqs provides an SQS SendMessage helper for VoltIQ telemetry.
// Replaces the original Kinesis producer — SQS is used because Kinesis
// requires an additional account subscription that may not be available
// on new AWS accounts, while SQS works immediately.
package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/voltiq/voltiq/internal/models"
)

// Client is an interface over *sqs.Client so this function
// can be unit-tested with a mock without hitting real AWS.
type Client interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// PutTelemetryEvent serialises event to JSON and sends it to queueURL.
// VehicleID is used as the MessageGroupId when FIFO queues are used,
// but standard queues (used here) don't require it.
func PutTelemetryEvent(ctx context.Context, client Client, queueURL string, event models.TelemetryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("sqs: marshal event for %s: %w", event.VehicleID, err)
	}

	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(data)),
	})
	if err != nil {
		return fmt.Errorf("sqs: SendMessage for %s: %w", event.VehicleID, err)
	}
	return nil
}
