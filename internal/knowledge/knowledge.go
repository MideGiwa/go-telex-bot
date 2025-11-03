package knowledge

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"go-telex-supabase-bot/internal/llm"
	"go-telex-supabase-bot/internal/supabase"
	"go-telex-supabase-bot/internal/telex" // Assumed this will be created soon
)

const (
	SourceTypeDoc  = "doc"
	SourceTypeChat = "chat"
)

// KnowledgeService handles ingestion of knowledge items into Supabase.
type KnowledgeService struct {
	geminiClient   *llm.GeminiClient
	supabaseClient *supabase.Client
	telexClient    *telex.Client // Assumed telex.Client will be available
	botUserID      string
}

// NewKnowledgeService creates a new KnowledgeService instance.
func NewKnowledgeService(gemini *llm.GeminiClient, sb *supabase.Client, tx *telex.Client, botUserID string) *KnowledgeService {
	return &KnowledgeService{
		geminiClient:   gemini,
		supabaseClient: sb,
		telexClient:    tx,
		botUserID:      botUserID,
	}
}

// IngestText processes and ingests a single text chunk into the knowledge base.
func (s *KnowledgeService) IngestText(ctx context.Context, content, sourceType, sourceURI string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("cannot ingest empty content")
	}

	embedding, err := s.geminiClient.EmbedContent(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to get embedding for content: %w", err)
	}

	item := &supabase.KnowledgeItem{
		Content:    content,
		Embedding:  embedding,
		SourceType: sourceType,
		SourceURI:  sourceURI,
		CreatedAt:  func() *time.Time { t := time.Now(); return &t }(),
	}

	if err := s.supabaseClient.InsertKnowledgeItem(ctx, item); err != nil {
		return fmt.Errorf("failed to insert knowledge item into Supabase: %w", err)
	}

	log.Printf("Successfully ingested item (Type: %s, URI: %s)", sourceType, sourceURI)
	return nil
}

// IngestDocumentFile reads a file and ingests its content as a document.
func (s *KnowledgeService) IngestDocumentFile(ctx context.Context, filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read document file %s: %w", filePath, err)
	}
	content := string(data)

	if len(strings.Fields(content)) < 10 { // Basic filter: skip very short documents
		log.Printf("Skipping document %s due to short content.", filePath)
		return nil
	}

	return s.IngestText(ctx, content, SourceTypeDoc, filePath)
}

// IngestTelexMessage processes a single Telex message for ingestion.
func (s *KnowledgeService) IngestTelexMessage(ctx context.Context, message *telex.Message) error {
	// Filter: Skip messages from the bot itself
	if message.SenderID == s.botUserID {
		log.Printf("Skipping bot's own message from channel %s, message ID %s", message.ChannelID, message.ID)
		return nil
	}

	// Filter: Skip messages shorter than 10 words (configurable)
	if len(strings.Fields(message.Content)) < 10 {
		log.Printf("Skipping short message from channel %s, message ID %s", message.ChannelID, message.ID)
		return nil
	}

	sourceURI := fmt.Sprintf("channel:%s/message:%s", message.ChannelID, message.ID)
	return s.IngestText(ctx, message.Content, SourceTypeChat, sourceURI)
}

// IngestHistoricalTelexChannelMessages fetches and ingests all messages from a given public Telex channel.
func (s *KnowledgeService) IngestHistoricalTelexChannelMessages(ctx context.Context, channelID string) error {
	if s.telexClient == nil {
		return fmt.Errorf("telex client not initialized for historical ingestion")
	}

	log.Printf("Fetching historical messages for channel: %s", channelID)
	messages, err := s.telexClient.GetChannelMessages(ctx, channelID) // Assumed GetChannelMessages exists
	if err != nil {
		return fmt.Errorf("failed to fetch historical messages for channel %s: %w", channelID, err)
	}

	log.Printf("Retrieved %d messages for channel %s. Starting ingestion...", len(messages), channelID)
	for _, msg := range messages {
		if err := s.IngestTelexMessage(ctx, &msg); err != nil { // Pass pointer to message
			log.Printf("Error ingesting historical message %s from channel %s: %v", msg.ID, msg.ChannelID, err)
			// Decide whether to continue or stop on error
		}
	}
	log.Printf("Finished ingesting historical messages for channel: %s", channelID)
	return nil
}

// ProcessDocsDirectory reads all .md files in the specified directory and ingests them.
func (s *KnowledgeService) ProcessDocsDirectory(ctx context.Context, docsDirPath string) error {
	log.Printf("Processing documents in directory: %s", docsDirPath)
	files, err := ioutil.ReadDir(docsDirPath)
	if err != nil {
		return fmt.Errorf("failed to read docs directory %s: %w", docsDirPath, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(docsDirPath, file.Name())
		if err := s.IngestDocumentFile(ctx, filePath); err != nil {
			log.Printf("Error ingesting document %s: %v", filePath, err)
			// Continue to next file even if one fails
		}
	}
	log.Printf("Finished processing documents in directory: %s", docsDirPath)
	return nil
}
