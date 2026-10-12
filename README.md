# Wuthering Waves Lore RAG System

A Retrieval-Augmented Generation (RAG) RESTful API web application designed to answer questions based on the Wuthering Waves game lore, specifically targeting the **"Simulacrum of the Heart"** main quest line.

The system features:
- **Automated Data Ingestion Pipeline**: Fetches raw wikitext from the Fandom MediaWiki API (`wutheringwaves.fandom.com/api.php`), cleans wiki markup, extracts dialogue turns, speakers, and objectives, and generates contextual chunks.
- **Semantic Vector Search**: Uses Qdrant vector database with 384-dimensional dense embeddings (`sentence-transformers/all-MiniLM-L6-v2`).
- **Resilient Multi-Tier LLM Gateway**: LiteLLM proxy container providing automatic routing and fallback from **Google Gemini API** (primary) to **Groq API / HuggingFace** (fallback) to handle free-tier API rate limits gracefully.
- **RESTful API**: Fast and lightweight Go web service with endpoints for Q&A chat, semantic search, status monitoring, and live re-ingestion.

---

## 1. Architecture & Tech Stack

```text
                               ┌────────────────────────┐
                               │  Wuthering Waves Wiki  │
                               │     (MediaWiki API)    │
                               └───────────┬────────────┘
                                           │
                                  [Fetcher & Parser]
                                           │
                                           ▼
┌──────────────────┐           ┌────────────────────────┐
│  Client / User   ├──────────►│   Go REST API Server   │
└──────────────────┘  /chat    │      (Gin Engine)      │
                               └─────┬────────────┬─────┘
                                     │            │
                         Vector Embed│            │LLM Prompt + Context
                                     ▼            ▼
                           ┌─────────────┐   ┌────────────────────────┐
                           │   Qdrant    │   │      LiteLLM Proxy     │
                           │  Vector DB  │   ├───────────┬────────────┤
                           │  (Port 6333)│   │  Primary  │  Fallback  │
                           └─────────────┘   │  Gemini   │    Groq    │
                                             └───────────┴────────────┘
```

* **Backend:** Golang 1.24+ (Gin Framework)
* **Data Source:** Fandom MediaWiki API (`https://wutheringwaves.fandom.com/api.php`)
* **Vector Database:** Qdrant (deployed via Docker / Podman)
* **LLM Gateway & Fallback:** LiteLLM Proxy (OpenAI-compatible `/v1/chat/completions`)
* **LLM Providers:** Google Gemini API (`gemini-1.5-flash`), Groq API (`llama-3.3-70b-versatile`)
* **Embedding Model:** `sentence-transformers/all-MiniLM-L6-v2` (384 dimensions)
* **Orchestration:** Docker Compose / Podman Compose

---

## 2. Project Directory Structure

```text
.
├── cmd/
│   ├── api/                 # Main entry point for REST API server
│   │   └── main.go
│   └── ingest/              # CLI tool for fetching and embedding wiki data
│       └── main.go
├── internal/
│   ├── config/              # Environment variables and configuration loader
│   │   └── config.go
│   ├── fetcher/             # MediaWiki API HTTP client & tests
│   │   ├── fetcher.go
│   │   └── fetcher_test.go
│   ├── parser/              # Wikitext cleaning, speaker extraction & chunking
│   │   ├── parser.go
│   │   └── parser_test.go
│   ├── qdrant/              # Qdrant vector database REST client & tests
│   │   ├── qdrant.go
│   │   └── qdrant_test.go
│   ├── rag/                 # RAG orchestration, embeddings & LiteLLM client
│   │   ├── embedding.go
│   │   ├── llm.go
│   │   ├── rag.go
│   │   └── rag_test.go
│   └── server/              # Gin HTTP handlers, routes & middleware
│       ├── server.go
│       └── server_test.go
├── deploy/
│   ├── docker-compose.yml   # Multi-service stack (qdrant, litellm, api)
│   └── litellm_config.yaml  # LiteLLM routing rules and fallback configuration
├── web/                   # Frontend prototype (static pages + app.js)
│   ├── index.html           # Landing / entry page
│   ├── ask.html             # Q&A chat view
│   ├── search.html          # Semantic search view
│   ├── admin*.html          # Admin console views (corpus, gateway, ingest, logs, settings)
│   ├── app.js               # Frontend logic (syntax-checked in CI via node --check)
│   ├── styles.css
│   └── context/             # Frontend source context notes
├── skills/                # Agent skills (design references)
│   ├── README.md
│   └── design-md/           # DESIGN.md analyses per product style
├── Dockerfile               # Multi-stage production container build
├── Makefile                 # Development shortcuts (build, test, run, ingest)
├── .env.example             # Template environment variables
├── go.mod
├── go.sum
└── plan.md
```

---

## 3. Quickstart

### Prerequisites
- [Go 1.24+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/engine/install/) or [Podman](https://podman.io/) with Compose support

### Step 1: Environment Setup
Copy `.env.example` to `.env` and provide your API keys:

```bash
cp .env.example .env
```

Edit `.env`:
```env
# Google Gemini API key (Primary)
GEMINI_API_KEY=your_gemini_api_key_here

# Groq API key (Fallback)
GROQ_API_KEY=your_groq_api_key_here

# HuggingFace API key (for embeddings)
HUGGINGFACE_API_KEY=your_huggingface_api_key_here
```

*(Note: If Hugging Face or LLM keys are not supplied during development, the system falls back to a deterministic semantic embedding and direct snippet retrieval gracefully).*

### Step 2: Running with Docker / Podman Compose

To launch the full stack (Qdrant + LiteLLM + API):

```bash
cd deploy
docker compose up -d
# or: podman compose up -d
```

Check running services:
```bash
docker compose ps
```

---

## 4. Ingesting Lore Data

### Preview Lore Parsing (Dry Run)
You can preview the extraction, speaker separation, and chunking without calling Qdrant or embedding APIs:

```bash
make dry-run
# Or: go run ./cmd/ingest/main.go --dry-run
```

### Full Ingestion into Qdrant
Ensure Qdrant is running, then execute:

```bash
make ingest
# Or: go run ./cmd/ingest/main.go --page "Simulacrum_of_the_Heart"
```

This will:
1. Fetch live wikitext for "Simulacrum of the Heart" from the MediaWiki API.
2. Clean all markup and split dialogue into structured scene blocks.
3. Compute 384-dimensional vector embeddings.
4. Create the `wuwa_lore` collection and batch upsert the points to Qdrant.

---

## 5. Running the REST API Locally

If Qdrant and LiteLLM are running, you can run the API server directly:

```bash
make run
# Or: go run ./cmd/api/main.go
```

The server listens on `http://localhost:8080`.

---

## 6. REST API Endpoints

### 1. Health Check
```http
GET /health
```
**Response:**
```json
{
  "status": "healthy",
  "service": "Wuthering Waves Lore RAG API",
  "qdrant_connected": true,
  "collection": "wuwa_lore",
  "indexed_chunks": 168,
  "timestamp": "2026-10-11T03:40:00Z"
}
```

### 2. Lore Q&A Chat (RAG)
```http
POST /api/v1/chat
Content-Type: application/json

{
  "question": "What is Lady Hsin's favorite treat according to the tea stall owner?"
}
```
**Response:**
```json
{
  "question": "What is Lady Hsin's favorite treat according to the tea stall owner?",
  "answer": "According to the Tea Stall Owner in Xianling Village, Lady Hsin's favorite treat is Moon Fox Rice Cakes.",
  "sources": [
    {
      "id": "016393b3-6370-43e8-448f-ca8b586826fc",
      "title": "Simulacrum of the Heart",
      "section": "Dialogue",
      "subsection": "Talk to the tea stall owner",
      "speakers": ["Tea Stall Owner", "Rover"],
      "content": "Tea Stall Owner: That'd be our new Moon Fox Rice Cakes, no question. They're Lady Hsin's favorite treat!",
      "score": 0.89
    }
  ],
  "model": "gemini/gemini-1.5-flash",
  "latency_ms": 420,
  "total_tokens": 185
}
```

### 3. Semantic Search Only
```http
POST /api/v1/search
Content-Type: application/json

{
  "query": "Moon Fox Rice Cakes",
  "limit": 3
}
```

### 4. Dynamic Ingestion via API
```http
POST /api/v1/ingest
Content-Type: application/json

{
  "page": "Simulacrum_of_the_Heart",
  "chunk_size": 900,
  "overlap": 150
}
```

### 5. Collection Status
```http
GET /api/v1/status
```

---

## 7. Automated Tests

Run all unit tests across internal packages:

```bash
make test
```

All test suites verify:
- Wikitext template parsing, dialogue tag normalization, and speaker attribution.
- Qdrant REST interaction (collection creation, vector upserts, and search queries).
- Embedding dimensionality and normalization.
- RAG context injection and LLM proxy calls.
- HTTP routing, CORS headers, and health endpoint status.
