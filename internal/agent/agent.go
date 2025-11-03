package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go-telex-supabase-bot/internal/llm"
	"go-telex-supabase-bot/internal/supabase"
	"go-telex-supabase-bot/internal/telex" // Assuming telex client is available
)

const (
	// SimilarityThreshold is the minimum similarity score for retrieved knowledge items.
	SimilarityThreshold = 0.75 // This should ideally match or be slightly below the Edge Function's threshold
	// MaxKnowledgeItems is the maximum number of knowledge items to retrieve.
	MaxKnowledgeItems = 5 // This should match the Edge Function's match_count
	// ContextWindowTokens is an approximate limit for the combined context to send to the LLM.
	// Adjust based on Gemini model context window capabilities.
	ContextWindowTokens = 8000 // Placeholder, actual token count would be more complex
)

// Agent orchestrates the RAG pipeline for answering user queries.
type Agent struct {
	geminiClient   *llm.GeminiClient
	supabaseClient *supabase.Client
	telexClient    *telex.Client
}

// NewAgent creates a new Agent instance.
func NewAgent(gemini *llm.GeminiClient, sb *supabase.Client, tx *telex.Client) *Agent {
	return &Agent{
		geminiClient:   gemini,
		supabaseClient: sb,
		telexClient:    tx,
	}
}

// AnswerQuestion performs the RAG pipeline to answer a user's question.
func (a *Agent) AnswerQuestion(ctx context.Context, channelID, query string) (string, error) {
	log.Printf("Agent: Received query \"%s\" for channel %s", query, channelID)

	// 1. Retrieve: Get embedding for the user's query
	queryVector, err := a.geminiClient.EmbedContent(ctx, query)
	if err != nil {
		return "", fmt.Errorf("agent: failed to get embedding for query: %w", err)
	}
	log.Printf("Agent: Generated embedding for query.")

	// 2. Retrieve: Invoke Supabase Edge Function for similarity search
	retrievalCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // Set a timeout for retrieval
	defer cancel()

	searchResults, err := a.supabaseClient.SearchKnowledge(retrievalCtx, queryVector)
	if err != nil {
		return "", fmt.Errorf("agent: failed to search knowledge base: %w", err)
	}
	log.Printf("Agent: Retrieved %d knowledge items.", len(searchResults))

	if len(searchResults) == 0 {
		log.Println("Agent: No relevant knowledge found.")
		return "I'm sorry, I couldn't find any relevant information to answer your question.", nil
	}

	// 3. Generate: Construct a detailed prompt for Gemini
	promptBuilder := strings.Builder{}
	promptBuilder.WriteString("Answer the user's question only based on the following context. Do not use outside knowledge. If the context does not contain enough information, state that you cannot answer the question based on the provided context.\n\n")
	promptBuilder.WriteString("Context:\n")

	for i, result := range searchResults {
		promptBuilder.WriteString(fmt.Sprintf("--- Source %d (Type: %s, URI: %s, Similarity: %.2f) ---\n", i+1, result.SourceType, result.SourceURI, result.Similarity))
		promptBuilder.WriteString(result.Content)
		promptBuilder.WriteString("\n\n")
	}

	promptBuilder.WriteString(fmt.Sprintf("User's question: %s\n", query))
	promptBuilder.WriteString("Answer:")

	systemInstruction := "You are a helpful assistant. Answer the user's question only based on the following context. Do not use outside knowledge."

	// 4. Generate: Call Gemini to generate the answer
	generationCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Set a timeout for generation
	defer cancel()

	generatedAnswer, err := a.geminiClient.GenerateContent(generationCtx, systemInstruction, promptBuilder.String())
	if err != nil {
		return "", fmt.Errorf("agent: failed to generate answer with Gemini: %w", err)
	}
	log.Printf("Agent: Generated answer with Gemini.")

	return generatedAnswer, nil
}

// RespondToTelex sends the generated answer back to the Telex channel.
func (a *Agent) RespondToTelex(ctx context.Context, channelID, message string) error {
	log.Printf("Agent: Sending response to channel %s: \"%s\"", channelID, message)
	err := a.telexClient.SendMessage(ctx, channelID, message)
	if err != nil {
		return fmt.Errorf("agent: failed to send message to Telex: %w", err)
	}
	log.Printf("Agent: Successfully sent response to channel %s.", channelID)
	return nil
}
