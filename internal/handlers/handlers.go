package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"go-telex-supabase-bot/internal/agent"
	"go-telex-supabase-bot/internal/knowledge"
	"go-telex-supabase-bot/internal/telex"

	"github.com/gin-gonic/gin"
)

// WebhookHandler handles incoming Telex webhooks.
type WebhookHandler struct {
	knowledgeService *knowledge.KnowledgeService
	ragAgent         *agent.Agent
	botUserID        string
}

// NewWebhookHandler creates a new WebhookHandler instance.
func NewWebhookHandler(ks *knowledge.KnowledgeService, ra *agent.Agent, botUserID string) *WebhookHandler {
	return &WebhookHandler{
		knowledgeService: ks,
		ragAgent:         ra,
		botUserID:        botUserID,
	}
}

// HandleTelexWebhook processes incoming Telex webhook events.
// It serves a dual purpose: real-time message ingestion and bot query answering.
func (h *WebhookHandler) HandleTelexWebhook(c *gin.Context) {
	var event telex.WebhookEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		log.Printf("Error binding JSON for webhook: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// CRITICAL: Respond with 200 OK immediately to Telex to avoid retries.
	c.JSON(http.StatusOK, gin.H{"status": "processing"})

	// Launch the actual processing in a goroutine with a timeout context
	// This ensures the goroutine doesn't hang indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	go func() {
		defer cancel()
		h.processWebhookEvent(ctx, event)
	}()
}

func (h *WebhookHandler) processWebhookEvent(ctx context.Context, event telex.WebhookEvent) {
	log.Printf("Processing Telex event_type: %s, message ID: %s", event.EventType, event.Payload.ID)

	switch event.EventType {
	case "message.created":
		message := &event.Payload
		if message == nil {
			log.Println("Received message.created event with nil payload.")
			return
		}

		// Validate message has required fields
		if message.ID == "" || message.ChannelID == "" {
			log.Printf("Received message.created event with missing required fields (ID: %s, ChannelID: %s)", message.ID, message.ChannelID)
			return
		}

		// Check if the bot is mentioned
		if h.isBotMentioned(message.Content) {
			cleanedQuery := h.cleanMention(message.Content)
			if strings.TrimSpace(cleanedQuery) == "" {
				log.Printf("Bot mentioned but query is empty after cleaning for message ID: %s", message.ID)
				return
			}
			log.Printf("Bot mentioned in channel %s, message ID %s. Query: \"%s\"", message.ChannelID, message.ID, cleanedQuery)

			// Launch RAG pipeline
			h.handleBotQuery(ctx, message.ChannelID, cleanedQuery)
		} else {
			// Regular message, launch ingestion logic
			h.handleRegularMessageIngestion(ctx, message)
		}
	// Add other event types handling if needed
	default:
		log.Printf("Unhandled Telex event type: %s", event.EventType)
	}
}

// handleRegularMessageIngestion handles non-mention messages for real-time learning.
func (h *WebhookHandler) handleRegularMessageIngestion(ctx context.Context, message *telex.Message) {
	ingestionCtx, cancel := context.WithTimeout(ctx, 15*time.Second) // Set a timeout for ingestion
	defer cancel()

	if err := h.knowledgeService.IngestTelexMessage(ingestionCtx, message); err != nil {
		log.Printf("Error ingesting real-time message %s from channel %s: %v", message.ID, message.ChannelID, err)
	} else {
		log.Printf("Successfully ingested real-time message %s from channel %s into knowledge base.", message.ID, message.ChannelID)
	}
}

// handleBotQuery handles messages where the bot is mentioned, triggering the RAG pipeline.
func (h *WebhookHandler) handleBotQuery(ctx context.Context, channelID, query string) {
	// The RAG pipeline itself handles its own timeouts for retrieval and generation steps.
	answer, err := h.ragAgent.AnswerQuestion(ctx, channelID, query)
	if err != nil {
		log.Printf("Error answering question in channel %s for query \"%s\": %v", channelID, query, err)
		// Attempt to send an error message back to the user
		err = h.ragAgent.RespondToTelex(ctx, channelID, "I encountered an error trying to answer your question. Please try again later.")
		if err != nil {
			log.Printf("Failed to send error message to Telex channel %s: %v", channelID, err)
		}
		return
	}

	err = h.ragAgent.RespondToTelex(ctx, channelID, answer)
	if err != nil {
		log.Printf("Error responding to Telex channel %s with answer: %v", channelID, err)
	}
}

// isBotMentioned checks if the bot is mentioned in the message content.
func (h *WebhookHandler) isBotMentioned(messageContent string) bool {
	mentionString := "<@" + h.botUserID + ">"
	return strings.Contains(messageContent, mentionString)
}

// cleanMention removes the bot mention from the message content.
func (h *WebhookHandler) cleanMention(messageContent string) string {
	mentionString := "<@" + h.botUserID + ">"
	cleaned := strings.ReplaceAll(messageContent, mentionString, "")
	return strings.TrimSpace(cleaned)
}
