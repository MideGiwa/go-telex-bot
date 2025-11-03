# Telex.im Self-Learning Knowledge Navigator Agent

This project implements an advanced Self-Learning Knowledge Navigator Agent for the Telex.im chat platform, built with Go, Gin, Supabase (Postgres + pgvector), and Google Gemini API. The agent intelligently answers user questions by learning from both static documents and real-time conversation history.

## Architecture V2 Overview

The V2 architecture introduces Supabase as the core "brain" of the agent, leveraging its Postgres database with the `pgvector` extension for efficient similarity search, and Edge Functions for secure retrieval.

### Core Components:

1.  **Supabase Backend:**
    *   **Postgres Database:** Stores `knowledge_items` (content, embeddings, source info).
    *   **`pgvector` Extension:** Enables vector similarity search.
    *   **`match_knowledge_items` Function:** A SQL function for performing vector similarity searches.
    *   **`search-knowledge` Edge Function (TypeScript/Deno):** A secure API endpoint for the Go agent to retrieve relevant knowledge items using the `match_knowledge_items` SQL function.

2.  **Go Ingestion Pipeline (The "Learning" System):**
    *   **Backfill CLI Tool (`cmd/backfill/main.go`):** Ingests historical data (static `.md` documents from `/docs` and historical Telex chat messages) into Supabase.
    *   **Real-Time Listener (`cmd/server/main.go`):** A Gin server endpoint that listens for new `message.created` webhooks from Telex. It processes new messages, generates embeddings, and inserts them into Supabase for continuous learning.

3.  **Go Agent (The "Answering" System):**
    *   **Gin Server (`cmd/server/main.go`):**
        *   Identifies when the bot is mentioned in a chat message.
        *   Triggers a **Retrieval-Augmented Generation (RAG)** pipeline in a goroutine to avoid blocking the webhook.
        *   **Retrieve:** Gets an embedding for the user's query from Gemini, then calls the Supabase `search-knowledge` Edge Function to fetch the most relevant knowledge items.
        *   **Generate:** Constructs a prompt with the retrieved context and sends it to the Gemini `generateContent` API to formulate an answer.
        *   **Respond:** Sends the generated answer back to the Telex channel.

## Project Structure

```
go-telex-supabase-bot/
├── cmd/
│   ├── backfill/
│   │   └── main.go              # CLI tool for historical data ingestion
│   └── server/
│       └── main.go              # Main Gin server (Agent & Real-time Listener)
├── supabase/
│   ├── functions/
│   │   └── search-knowledge/
│   │       └── index.ts         # Supabase Edge Function for RAG retrieval
│   └── schema.sql               # Supabase database schema (table, pgvector, search function)
├── internal/
│   ├── agent/                   # Contains the RAG (Answering) pipeline logic
│   ├── config/                  # Loads .env file configuration
│   ├── handlers/                # Gin webhook handler logic
│   ├── knowledge/               # Service for ingesting data into Supabase
│   ├── llm/                     # Google Gemini client wrapper
│   ├── supabase/                # Supabase Go client wrapper
│   └── telex/                   # Telex API client wrapper
├── docs/
│   └── company-info.md          # Sample static documents for ingestion
├── go.mod                       # Go module definition
├── .env.example                 # Example environment variables
└── README.md                    # This file
```

## Setup Instructions

### 1. Supabase Project Setup

1.  **Create a Supabase Project:**
    *   Go to [Supabase](https://supabase.com/) and create a new project.
2.  **Enable `pgvector` Extension:**
    *   In your Supabase project dashboard, navigate to **Database > Extensions**.
    *   Search for `vector` and enable it.
3.  **Apply Database Schema:**
    *   Go to **SQL Editor**.
    *   Copy the content of `supabase/schema.sql` and paste it into the SQL editor.
    *   Run the query to create the `knowledge_items` table and the `match_knowledge_items` function.
4.  **Deploy Edge Function:**
    *   Ensure you have the [Supabase CLI](https://supabase.com/docs/guides/cli) installed.
    *   Navigate to your Supabase project root (where `supabase` directory is).
    *   Run `supabase functions new search-knowledge` (this creates a boilerplate, you'll replace its content).
    *   Replace the content of the generated `supabase/functions/search-knowledge/index.ts` with the content from this project's `supabase/functions/search-knowledge/index.ts`.
    *   Deploy the function: `supabase functions deploy search-knowledge --no-verify-jwt`.
    *   **Important:** The Edge Function requires the `SUPABASE_SERVICE_ROLE_KEY` to be accessible. Ensure it's set as a [Supabase Secret](https://supabase.com/docs/guides/functions/secrets) for your Edge Function.

### 2. Environment Variables

1.  Copy `.env.example` to `.env`:
    ```bash
    cp .env.example .env
    ```
2.  Fill in the values in your `.env` file:
    *   `GEMINI_API_KEY`: Obtain this from the [Google AI Studio](https://aistudio.google.com/).
    *   `TELEX_API_BASE_URL`: The base URL for your Telex API instance (e.g., `http://localhost:8080/api/v1`).
    *   `TELEX_BOT_USER_ID`: The user ID of your bot on Telex.im (e.g., `U12345678`).
    *   `SUPABASE_URL`: Find this in your Supabase project settings under **Project Settings > API > Project URL**.
    *   `SUPABASE_SERVICE_KEY`: Find this in your Supabase project settings under **Project Settings > API > Service Role (secret)**. **Be careful not to expose this key.**

### 3. Go Project Setup

1.  Navigate into the `go-telex-supabase-bot` directory:
    ```bash
    cd go-telex-supabase-bot
    ```
2.  Download Go modules:
    ```bash
    go mod tidy
    ```

### 4. Running the Backfill CLI Tool (Initial Ingestion)

This tool will populate your Supabase `knowledge_items` table with initial data.

```bash
go run cmd/backfill/main.go
```
*This will read files from the `docs/` directory and fetch historical messages from the Telex API (if configured correctly).*

### 5. Running the Gin Server (Agent & Real-time Listener)

This will start the main server that handles real-time message ingestion and answers user queries.

```bash
go run cmd/server/main.go
```

### 6. Configure Telex Webhooks

Your Telex.im instance needs to be configured to send webhooks to your running Gin server.

*   **Webhook URL:** `http://your_server_address:port/api/v1/telex/webhook` (e.g., `http://localhost:8081/api/v1/telex/webhook` if running locally).
*   **Events to subscribe to:** `message.created`

## Usage

Once both the `backfill` tool has run and the `server` is running, your agent will be operational:

*   **Real-time Learning:** Any new messages posted in public Telex channels (that meet the filtering criteria, e.g., >10 words) will be automatically ingested and learned by the bot.
*   **Answering Queries:** Mention your bot in a Telex channel (e.g., `@KnowledgeBot What is our VPN policy?`). The bot will retrieve relevant information from its knowledge base (documents and past chat history) and generate an answer using Gemini.