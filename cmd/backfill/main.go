package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"

	"go-telex-supabase-bot/internal/config"
	"go-telex-supabase-bot/internal/knowledge"
	"go-telex-supabase-bot/internal/llm"
	"go-telex-supabase-bot/internal/supabase"
	"go-telex-supabase-bot/internal/telex"
)

const (
	docsDirPath = "docs" // Relative path to the docs directory
)

func main() {
	log.Println("Starting backfill CLI tool...")

	cfg := config.LoadConfig()

	// Initialize LLM Client (Gemini)
	geminiClient, err := llm.NewGeminiClient(cfg.GeminiAPIKey)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}
	defer func() {
		if err := geminiClient.Close(); err != nil {
			log.Printf("Error closing Gemini client: %v", err)
		}
	}()
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

	// Initialize Knowledge Service
	knowledgeService := knowledge.NewKnowledgeService(geminiClient, supabaseClient, telexClient, cfg.TelexBotUserID)
	log.Println("Knowledge service initialized.")

	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Part 1: Ingest static .md files from the /docs folder
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Starting document ingestion from %s...", docsDirPath)
		// Get the absolute path to the docs directory
		absDocsDirPath, err := filepath.Abs(docsDirPath)
		if err != nil {
			log.Printf("Error getting absolute path for docs directory: %v", err)
			return
		}

		// Ensure the directory exists
		if _, err := os.Stat(absDocsDirPath); os.IsNotExist(err) {
			log.Printf("Docs directory not found at %s. Skipping document ingestion.", absDocsDirPath)
			return
		}

		if err := knowledgeService.ProcessDocsDirectory(ctx, absDocsDirPath); err != nil {
			log.Printf("Error during document ingestion: %v", err)
		} else {
			log.Println("Document ingestion completed.")
		}
	}()

	// Part 2: Ingest historical public channel messages from Telex
	// NOTE: This part requires identifying public channels. For a real system,
	// you'd typically fetch a list of public channels from the Telex API or
	// have them configured. For this example, we'll use a placeholder.
	publicChannelIDs := []string{
		"C12345678", // Example: Replace with actual public channel IDs
		"C98765432", // Example: Replace with actual public channel IDs
	}

	if len(publicChannelIDs) == 0 {
		log.Println("No public Telex channel IDs configured for historical ingestion. Skipping chat history ingestion.")
	} else {
		for _, channelID := range publicChannelIDs {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				log.Printf("Starting historical chat message ingestion for channel %s...", id)
				if err := knowledgeService.IngestHistoricalTelexChannelMessages(ctx, id); err != nil {
					log.Printf("Error during historical chat message ingestion for channel %s: %v", id, err)
				} else {
					log.Printf("Historical chat message ingestion for channel %s completed.", id)
				}
			}(channelID)
		}
	}

	wg.Wait()
	log.Println("Backfill CLI tool finished all ingestion tasks.")
}
