// Package bedrock wraps AWS Bedrock InvokeModel for Amazon Nova Lite
// and provides prompt-building and JSON response parsing for charging decisions.
//
// NOTE: We use Amazon Nova Lite (amazon.nova-lite-v1:0) rather than Claude Haiku
// because Nova models are auto-enabled on every AWS account with no use-case form.
// Claude models require a one-time Anthropic approval step that can block deployment.
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/voltiq/voltiq/internal/models"
)

const (
	// modelID is Amazon Nova Lite — fast, capable, no approval form required.
	modelID = "amazon.nova-lite-v1:0"

	// systemPrompt instructs the model to act as VoltIQ and respond only in JSON.
	systemPrompt = `You are VoltIQ, an AI charging optimizer for Nigerian electric vehicle fleets operating in Lagos, Nigeria.

BATTERY URGENCY RULES — you MUST follow these exactly:
- CHARGE_NOW : battery is below 20% OR the vehicle cannot reach its next destination. This is CRITICAL. Always use CHARGE_NOW when battery < 20%.
- CHARGE_LATER: battery is between 20% and 50%. The vehicle is safe for now but should be scheduled to charge during the next off-peak window (23:00–06:00 WAT) to save money.
- SKIP        : battery is above 50%. No charging action is needed.

Nigerian tariff tiers (WAT = UTC+1):
- Shoulder  06:00–18:00 WAT (225 NGN/kWh)
- Peak      18:00–23:00 WAT (320 NGN/kWh) — avoid charging now, schedule for off-peak
- Off-peak  23:00–06:00 WAT (185 NGN/kWh) — cheapest, always prefer this window

Always respond with valid JSON only. No text outside the JSON object.`
)

// Amazon Nova API request and response definitions.

// novaRequest is the InvokeModel body for Amazon Nova models.
type novaRequest struct {
	SchemaVersion   string        `json:"schemaVersion"`
	Messages        []novaMessage `json:"messages"`
	System          []novaText    `json:"system"`
	InferenceConfig novaInference `json:"inferenceConfig"`
}

type novaMessage struct {
	Role    string     `json:"role"`
	Content []novaText `json:"content"`
}

type novaText struct {
	Text string `json:"text"`
}

type novaInference struct {
	MaxNewTokens int     `json:"max_new_tokens"`
	Temperature  float64 `json:"temperature"`
}

// novaResponse is the top-level InvokeModel response for Amazon Nova.
type novaResponse struct {
	Output struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	} `json:"output"`
}

// BedrockClient handles execution of prompts on AWS Bedrock runtime.

// BedrockClient wraps the AWS Bedrock runtime client.
type BedrockClient struct {
	client *bedrockruntime.Client
}

// NewBedrockClient creates a BedrockClient using the given AWS region.
// Credentials are loaded from the Lambda execution role environment.
func NewBedrockClient(region string) (*BedrockClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load config: %w", err)
	}
	return &BedrockClient{
		client: bedrockruntime.NewFromConfig(cfg),
	}, nil
}

// Invoke sends userPrompt to Amazon Nova Lite and returns the raw text response.
// The response is expected to be a JSON object as instructed by systemPrompt.
func (b *BedrockClient) Invoke(userPrompt string) (string, error) {
	reqBody := novaRequest{
		SchemaVersion: "messages-v1",
		System:        []novaText{{Text: systemPrompt}},
		Messages: []novaMessage{
			{
				Role:    "user",
				Content: []novaText{{Text: userPrompt}},
			},
		},
		InferenceConfig: novaInference{
			MaxNewTokens: 512,
			Temperature:  0.1, // low temperature for consistent JSON output
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("bedrock: marshal request: %w", err)
	}

	result, err := b.client.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Body:        bodyBytes,
	})
	if err != nil {
		return "", fmt.Errorf("bedrock: invoke model: %w", err)
	}

	var resp novaResponse
	if err := json.Unmarshal(result.Body, &resp); err != nil {
		return "", fmt.Errorf("bedrock: unmarshal response: %w", err)
	}
	if len(resp.Output.Message.Content) == 0 {
		return "", fmt.Errorf("bedrock: empty content in response")
	}

	return resp.Output.Message.Content[0].Text, nil
}

// ParseDecision parses a raw Nova text response into a ChargingDecision.
// It strips markdown code fences that the model sometimes adds, then unmarshals JSON.
// vehicleID is injected into the result in case the model omits it.
func (b *BedrockClient) ParseDecision(raw string, vehicleID string) models.ChargingDecision {
	clean := strings.TrimSpace(raw)

	// Strip leading ```json or ``` fences
	if strings.HasPrefix(clean, "```json") {
		clean = strings.TrimPrefix(clean, "```json")
	} else if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
	}
	// Strip trailing fence
	clean = strings.TrimSuffix(strings.TrimSpace(clean), "```")
	clean = strings.TrimSpace(clean)

	var decision models.ChargingDecision
	if err := json.Unmarshal([]byte(clean), &decision); err != nil {
		// Return a safe default so the pipeline doesn't fail
		return models.ChargingDecision{
			VehicleID: vehicleID,
			Action:    "SKIP",
			Reasoning: fmt.Sprintf("AI parse error: %v — raw: %s", err, raw),
		}
	}

	// Always inject vehicleID — model may omit it
	decision.VehicleID = vehicleID
	return decision
}
