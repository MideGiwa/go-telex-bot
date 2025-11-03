package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	gosupabase "github.com/supabase-community/supabase-go"
	"github.com/supabase-community/supabase-go/storage"
)

// Client wraps the Supabase Go client and provides additional functionality.
type Client struct {
	*gosupabase.Client
	supabaseURL        string
	supabaseServiceKey string
}

// KnowledgeItem represents a row in the knowledge_items table.
type KnowledgeItem struct {
	ID         uuid.UUID  `json:"id,omitempty"`
	Content    string     `json:"content"`
	Embedding  []float32  `json:"embedding"`
	SourceType string     `json:"source_type"`
	SourceURI  string     `json:"source_uri"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

// SearchResult represents a single result from the knowledge search.
type SearchResult struct {
	ID         uuid.UUID `json:"id"`
	Content    string    `json:"content"`
	SourceType string    `json:"source_type"`
	SourceURI  string    `json:"source_uri"`
	Similarity float64   `json:"similarity"`
}

// NewClient initializes and returns a new Supabase client wrapper.
func NewClient(supabaseURL, supabaseServiceKey string) (*Client, error) {
	supabaseClient, err := gosupabase.NewClient(supabaseURL, supabaseServiceKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	// Ping the database to ensure connection (optional, but good for early error detection)
	// In gosupabase, direct ping isn't exposed, but we can try a simple query.
	// For production, a more robust health check might be needed or rely on subsequent operations.
	// storageClient := storage.NewClient(supabaseURL, supabaseServiceKey, nil)
	// _, err = storageClient.ListBuckets()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to connect to Supabase storage (check URL/Key): %w", err)
	// }

	return &Client{
		Client:             supabaseClient,
		supabaseURL:        supabaseURL,
		supabaseServiceKey: supabaseServiceKey,
	}, nil
}

// InsertKnowledgeItem inserts a new knowledge item into the knowledge_items table.
func (c *Client) InsertKnowledgeItem(ctx context.Context, item *KnowledgeItem) error {
	_, _, err := c.DB.From("knowledge_items").Insert(item, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to insert knowledge item: %w", err)
	}
	return nil
}

// SearchKnowledge calls the Supabase Edge Function to perform a vector similarity search.
func (c *Client) SearchKnowledge(ctx context.Context, queryVector []float32) ([]SearchResult, error) {
	edgeFunctionURL := fmt.Sprintf("%s/functions/v1/search-knowledge", c.supabaseURL)

	payload := map[string][]float32{"query_vector": queryVector}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query vector: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", edgeFunctionURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for Edge Function: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// For Edge Functions, the Authorization header typically uses the anon key,
	// unless the function is set to require a service role key.
	// Given the instruction to use `SUPABASE_SERVICE_ROLE_KEY` for retrieval function,
	// and to deploy with `--no-verify-jwt`, it implies we pass the service key.
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.supabaseServiceKey))

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Supabase Edge Function: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Supabase Edge Function returned non-200 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from Edge Function: %w", err)
	}

	var results []SearchResult
	if err := json.Unmarshal(bodyBytes, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Edge Function response: %w, raw body: %s", err, string(bodyBytes))
	}

	return results, nil
}

// GetStorageClient returns a Supabase Storage client.
// This might be useful for future expansion or if direct file storage interaction is needed.
func (c *Client) GetStorageClient() *storage.Client {
	return storage.NewClient(c.supabaseURL, c.supabaseServiceKey, nil)
}

// UpsertKnowledgeItem is a helper to either insert or update based on a unique identifier
// (not strictly required by current schema but good for future proofing/deduplication).
// For now, we'll just use Insert. If deduplication is needed, a `source_uri` unique constraint
// and `ON CONFLICT` clause might be added to the schema.
/*
func (c *Client) UpsertKnowledgeItem(ctx context.Context, item *KnowledgeItem) error {
	// Example: assuming source_uri could be a unique identifier
	// This would require an `ON CONFLICT` clause in the SQL.
	// For now, we only have `InsertKnowledgeItem` which always adds a new entry.
	// To implement upsert, you'd typically need a unique constraint on source_uri
	// and use the `OnConflict` option in the Supabase-Go client's Insert method.
	_, _, err := c.DB.From("knowledge_items").
		Insert(item, false, "source_uri", "id,content,embedding,source_type,source_uri", "").
		Execute()
	if err != nil {
		return fmt.Errorf("failed to upsert knowledge item: %w", err)
	}
	return nil
}
*/
