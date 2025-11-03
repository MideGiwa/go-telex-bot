package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go-telex-supabase-bot/internal/agent"
	"go-telex-supabase-bot/internal/config"
	"go-telex-supabase-bot/internal/handlers"
	"go-telex-supabase-bot/internal/knowledge"
	"go-telex-supabase-bot/internal/llm"
	"go-telex-supabase-bot/internal/supabase"
	"go-telex-supabase-bot/internal/telex"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting Gin server (Agent & Real-time Listener)...")

	cfg := config.LoadConfig()

	// Initialize LLM Client (Gemini)
	geminiClient, err := llm.NewGeminiClient(cfg.GeminiAPIKey)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}
	log.Println("Gemini client initialized.")

	// Initialize Supabase Client
	supabaseClient, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceKey)
	if err != nil {
		log.Fatalf("Failed to initialize Supabase client: %v", err)
	}
	log.Println("Supabase client initialized.")

	// Initialize Telex Client
	telexClient := telex.NewClient(cfg.TelexAPIBaseURL)
	log.Println("Telex client initialized.")

	// Initialize Knowledge Service (for real-time ingestion)
	knowledgeService := knowledge.NewKnowledgeService(geminiClient, supabaseClient, telexClient, cfg.TelexBotUserID)
	log.Println("Knowledge service initialized for real-time ingestion.")

	// Initialize Agent (for RAG pipeline)
	ragAgent := agent.NewAgent(geminiClient, supabaseClient, telexClient)
	log.Println("RAG Agent initialized.")

	router := gin.Default()

	// Register the webhook handler
	webhookHandler := handlers.NewWebhookHandler(knowledgeService, ragAgent, cfg.TelexBotUserID)
	router.POST("/api/v1/telex/webhook", webhookHandler.HandleTelexWebhook)

	srv := &http.Server{
		Addr:    ":8081", // You can configure this port
		Handler: router,
	}

	// Start server in a goroutine so that it doesn't block the graceful shutdown logic
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("Gin server listening on %s", srv.Addr)

	// Wait for interrupt signal to gracefully shut down the server with a timeout
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// isBotMentioned checks if the bot is mentioned in the message content.
// This is a basic implementation; more robust parsing might be needed for complex cases.
func isBotMentioned(messageContent, botUserID string) bool {
	// Telex often uses a format like <@U12345678> for mentions.
	// We'll check for the bot's user ID in this format.
	mentionString := "<@" + botUserID + ">"
	return strings.Contains(messageContent, mentionString)
}

// cleanMention removes the bot mention from the message content.
func cleanMention(messageContent, botUserID string) string {
	mentionString := "<@" + botUserID + ">"
	cleaned := strings.ReplaceAll(messageContent, mentionString, "")
	return strings.TrimSpace(cleaned)
}
