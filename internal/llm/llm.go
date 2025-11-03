package llm

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	// GeminiEmbeddingModel is the model used for generating embeddings.
	GeminiEmbeddingModel = "embedding-001"
	// GeminiGenerationModel is the model used for generating content.
	GeminiGenerationModel = "gemini-1.5-flash-latest" // or gemini-2.5-flash-preview-09-2025 as per instruction, using latest for now
)

// GeminiClient provides methods for interacting with the Google Gemini API.
type GeminiClient struct {
	embeddingModel *genai.GenerativeModel
	generationModel *genai.GenerativeModel
}

// NewGeminiClient initializes and returns a new GeminiClient.
func NewGeminiClient(apiKey string) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Initialize embedding model
	embeddingModel := client.GenerativeModel(GeminiEmbeddingModel)

	// Initialize generation model
	generationModel := client.GenerativeModel(GeminiGenerationModel)
	generationModel.SetTemperature(0.7) // Sensible default for general use

	return &GeminiClient{
		embeddingModel: embeddingModel,
		generationModel: generationModel,
	}, nil
}

// EmbedContent generates a vector embedding for the given text.
func (c *GeminiClient) EmbedContent(ctx context.Context, text string) ([]float32, error) {
	if c.embeddingModel == nil {
		return nil, errors.New("Gemini embedding model not initialized")
	}

	res, err := c.embeddingModel.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding from Gemini: %w", err)
	}

	if res == nil || len(res.Embedding) == 0 || len(res.Embedding[0].Values) == 0 {
		return nil, errors.New("received empty or invalid embedding from Gemini")
	}

	return res.Embedding[0].Values, nil
}

// GenerateContent generates text based on a prompt and provided context.
// The systemInstruction is optional.
func (c *GeminiClient) GenerateContent(ctx context.Context, systemInstruction string, prompt string) (string, error) {
	if c.generationModel == nil {
		return nil, errors.New("Gemini generation model not initialized")
	}

	model := c.generationModel

	// Add system instruction if provided
	var parts []genai.Part
	if systemInstruction != "" {
		parts = append(parts, genai.Text(systemInstruction))
	}
	parts = append(parts, genai.Text(prompt))

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from Gemini: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return "", errors.New("received no candidates from Gemini")
	}

	var fullResponse string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			fullResponse += string(text)
		}
	}

	if fullResponse == "" {
		log.Printf("Warning: Generated content was empty for prompt: %s", prompt)
	}

	return fullResponse, nil
}
