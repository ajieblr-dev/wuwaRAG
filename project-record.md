# Wuthering Waves Lore RAG System — Comprehensive Project Record

**Project Repository:** [https://github.com/ajieblr-dev/wuwaRAG](https://github.com/ajieblr-dev/wuwaRAG)  
**Target Lore:** Wuthering Waves — Main Quest *"Simulacrum of the Heart"* & Solaris-3 World Lore  
**Documentation Version:** 1.0.0 (October 2026)  
**Audience:** Incoming Developers, Maintainers, and Systems Engineers  

---

## 1. Executive Summary & Architecture Overview

The **Wuthering Waves Lore RAG System** is an end-to-end Retrieval-Augmented Generation (RAG) platform designed to provide accurate, grounded answers to complex lore inquiries regarding Wuthering Waves, with special focus on the **"Simulacrum of the Heart"** quest line (Version 3.7+), Sentinel Hsin, Intendant Suoming, and the city of Xuanfang in Mengzhou.

### Key Capabilities
1. **Automated Live Wiki Ingestion:** Queries the Fandom MediaWiki API (`wutheringwaves.fandom.com/api.php`), parses raw wikitext, extracts narrative dialogue turns and speaker tags, and structures them into contextual chunks.
2. **Persistent Raw & Processed Data Caching:** Saves raw JSON wiki dumps (`data/raw/`) and parsed chunk structures (`data/processed/`) for reproducible offline processing and version control.
3. **Dense Vector Search with Qdrant:** Generates 384-dimensional dense vector embeddings (`sentence-transformers/all-MiniLM-L6-v2`) and manages similarity indexing in Qdrant using Cosine distance.
4. **Multi-Tier LLM Gateway & Fallback:** Uses a **LiteLLM** proxy container routing to **Google Gemini 3.5 Flash** (primary) and **Gemini 3.1 Flash-Lite** / **Groq Llama 3.3 70B** (fallback), with an automatic in-app retrieval fallback if external gateways are unavailable.
5. **High-Performance Go REST API:** Built with the Gin framework, exposing endpoints for semantic search, full RAG chat, health monitoring, and on-demand live wiki re-ingestion.

```
                              ┌───────────────────────────────────┐
                              │    Fandom MediaWiki API           │
                              │   (wutheringwaves.fandom.com)     │
                              └─────────────────┬─────────────────┘
                                                │
                                       [Fetcher & Parser]
                                                │
                                                ▼
┌──────────────────┐           ┌───────────────────────────────────┐
│   User / Client  ├──────────►│      Go REST API Server (Gin)     │
└──────────────────┘  /chat    │         (Port 8080)               │
                               └────────┬───────────────────┬──────┘
                                        │                   │
                                Vector  │                   │ Prompt + Injected
                                Search  ▼                   ▼ Context
                       ┌─────────────────────────┐ ┌─────────────────────────┐
                       │   Qdrant Vector DB      │ │   LiteLLM Proxy (4000)  │
                       │   Collection: wuwa_lore │ ├───────────┬─────────────┤
                       │   (Port 6333 / 6334)    │ │  Primary  │  Fallback   │
                       └─────────────────────────┘ │  Gemini   │  Gemini-Lite│
                                                   │ 3.5 Flash │ / Groq 70B  │
                                                   └───────────┴─────────────┘
```

---

## 2. Chronological Process & Implementation Log

### Phase 1: Environment Setup & Repository Alignment
* **Operating System Audit:** The environment is Linux (Nobara 44 x86_64) equipped with Docker 29.9 and Podman.
* **Go Toolchain Setup:** Installed Go locally into `~/.local/go` (Go 1.27.2) and configured binary symlinks to PATH (`~/.local/bin/go`).
* **Module & Git Configuration:**
  - Initialized Go module under `github.com/ajieblr-dev/wuwaRAG`.
  - Configured Git remote `origin` pointing to `https://github.com/ajieblr-dev/wuwaRAG.git` with primary branch `main`.
  - Created `.gitignore` ignoring binary artifacts, OS artifacts, and environment secrets (`.env`).

### Phase 2: Core Components Development (`internal/`)
We implemented the internal modules specified in `plan.md`:

1. **Configuration Loader ([`internal/config/config.go`](file:///home/ajibul/WorkingDirectory/wuwaRAG/internal/config/config.go)):**
   - Automatically loads `.env` via `godotenv`.
   - Manages configurations for ports, Qdrant URLs, LiteLLM URLs, API keys, and RAG thresholds.
2. **MediaWiki Fetcher ([`internal/fetcher/fetcher.go`](file:///home/ajibul/WorkingDirectory/wuwaRAG/internal/fetcher/fetcher.go)):**
   - Implements `FetchPage`, `FetchSection`, `SearchPages`, `SavePageToDisk`, and `LoadPageFromDisk`.
   - Sends compliant `User-Agent` headers (`WuwaLoreRAG/1.0`).
   - Covered with unit tests in `fetcher_test.go` using mock HTTP test servers.
3. **Wikitext Parser & Chunker ([`internal/parser/parser.go`](file:///home/ajibul/WorkingDirectory/wuwaRAG/internal/parser/parser.go)):**
   - Cleans wiki markup templates: `{{Rover}}`, `{{DIcon}}`, `{{WavesLine}}`, `{{Quest Description}}`, `{{Black Screen}}`.
   - Normalizes objective headers (`;Objective` -> `[Objective: ...]`).
   - Parses dialogue speaker turns and populates a metadata array of speakers (`ExtractSpeakers`).
   - Generates deterministic UUIDs for each chunk to prevent duplication across re-ingestion runs.
   - Tested in `parser_test.go`.
4. **Qdrant Vector Database REST Client ([`internal/qdrant/qdrant.go`](file:///home/ajibul/WorkingDirectory/wuwaRAG/internal/qdrant/qdrant.go)):**
   - Health checking (`/readyz`, `/healthz`).
   - Collection creation (`EnsureCollection`) configured for 384 dimensions with Cosine distance.
   - Batch point upsertion (`UpsertPoints`) with retry resilience.
   - Semantic similarity search (`Search`) with score thresholds.
   - Tested in `qdrant_test.go`.
5. **RAG Pipeline & LLM Gateway ([`internal/rag/`](file:///home/ajibul/WorkingDirectory/wuwaRAG/internal/rag/)):**
   - `embedding.go`: Supports HuggingFace Inference API (`sentence-transformers/all-MiniLM-L6-v2`) and includes a deterministic normalized fallback embedder for offline testing.
   - `llm.go`: OpenAI-compatible client targeting LiteLLM, plus direct fallback to Gemini/Groq APIs.
   - `rag.go`: Orchestrator that accepts a question, runs vector retrieval, formats structured context blocks, and calls the LLM gateway.
   - Tested in `rag_test.go`.
6. **REST API Server ([`internal/server/server.go`](file:///home/ajibul/WorkingDirectory/wuwaRAG/internal/server/server.go)):**
   - Gin web framework with CORS middleware, recovery, and logging.
   - Handlers for `/health`, `/api/v1/status`, `/api/v1/chat`, `/api/v1/search`, and `/api/v1/ingest`.
   - Tested in `server_test.go`.

### Phase 3: Data Gathering from MediaWiki
We executed the live data gathering against `wutheringwaves.fandom.com`:
* **5 Target Pages Gathered:**
  - `Simulacrum of the Heart` (Main Quest): 168,893 characters
  - `Hsin` (Sentinel Lore): 3,167 characters
  - `Suoming` (Intendant Lore): 1,861 characters
  - `Mengzhou` (Region Lore): 5,179 characters
  - `Xuanfang Hold` (City Lore): 4,028 characters
* **Raw Storage:** Cached in [`data/raw/*.json`](file:///home/ajibul/WorkingDirectory/wuwaRAG/data/raw/).
* **Chunk Generation:** Generated **222 structured chunks** stored in [`data/processed/chunks.json`](file:///home/ajibul/WorkingDirectory/wuwaRAG/data/processed/chunks.json).
* **Speaker Identification:** Automatically identified 71 distinct speakers across all dialogue scenes (Rover, Suoming, Hsin, Abby, Tangning, Qingxiao, Jiu, Ninth Seal, etc.).

### Phase 4: Docker Container Deployment & Vector Indexing
* Created `deploy/docker-compose.yml` and `Dockerfile` (multi-stage Go build).
* Started containers:
  - `wuwa-qdrant`: Port `6333` (REST) and `6334` (gRPC).
  - `wuwa-litellm`: Port `4000` (OpenAI proxy).
* Ran vector ingestion (`./bin/ingest --from-cache`):
  - Created Qdrant collection `wuwa_lore`.
  - Generated 222 384-dimensional dense vectors.
  - Successfully upserted all points to Qdrant (Status: `green`, Count: `222`).

### Phase 5: Gemini LLM Integration & Crucial Debugging Insights
When connecting the Google Gemini API key, we addressed two key architectural challenges that incoming developers should understand:

#### 1. Gemini Model Lifecycle & Deprecation
* **Observation:** The default `gemini-1.5-flash` model returned `404 Not Found` for newly generated API keys.
* **Root Cause:** In modern Google AI Studio environments, older model IDs (`gemini-1.5-flash`, `gemini-2.5-flash`) have been deprecated for new projects.
* **Resolution:** By querying `ModelService.ListModels`, we determined that the active supported models are **`gemini-3.5-flash`** and **`gemini-3.1-flash-lite`**.
* **Action:** Configured `deploy/litellm_config.yaml` to route `wuwa-lore-model` to `gemini/gemini-3.5-flash` with fallback to `gemini/gemini-3.1-flash-lite`.

#### 2. Chain-of-Thought Reasoning Token Budget
* **Observation:** The initial RAG chat test cut off after just one sentence.
* **Root Cause:** `gemini-3.5-flash` is an advanced reasoning model that produces internal thinking/reasoning tokens. Out of an initial `max_tokens: 1024` limit, 981 tokens were consumed by internal reasoning, leaving only ~40 tokens for text output, triggering `finish_reason: length`.
* **Resolution:** 
  - Increased `max_tokens` from `1024` to `4096` in `internal/rag/llm.go`.
  - Increased server request timeout from `60s` to `120s` in `internal/server/server.go`.
* **Result:** The system now generates rich, comprehensive, multi-paragraph answers with grounded citations.

### Phase 6: Frontend Web Application Integration (`web/`)
To deliver a complete interactive user experience, the system integrates a modern lore-themed web client located in `web/` directly with the Go REST API:
* **Static File Serving & Routing:** Updated `internal/server/server.go` with dynamic directory discovery (`findWebDir()`) to serve `index.html`, `ask.html`, `search.html`, `styles.css`, `app.js`, and fonts directly from port `:8080`.
* **Wire Contract Parity:**
  - Aliased `SourceSnippet` JSON tags in `internal/rag/rag.go` (`page_title`, `section_title`, `chunk_text`) to support both legacy and web frontend consumer specifications without breaking changes.
  - Added `limit` echo in search response envelopes.
* **Auto Mode Detection:** Updated `web/app.js` so that when accessed over HTTP/HTTPS, it defaults `mock: false` and uses the host origin (`http://localhost:8080`), automatically switching from prototype mode to live production RAG queries.
* **Docker Packaging:** Integrated `web/` into `Dockerfile` and `deploy/docker-compose.yml` so production containers ship with the UI out-of-the-box.
* **Automated Verification:** Added `scripts/smoke-web.sh` and Go contract tests (`TestChatContractMatchesWebExpectations`, `TestSearchContractMatchesWebExpectations`, `TestStaticWebRoutes`) in `internal/server/server_test.go`.

### Phase 7: Full Main Quest Corpus Expansion (Prologue through Chapter IV)
To dramatically increase the model's lore knowledge across all released storyline chapters:
* **Corpus Scope:** Curated and ingested **78 comprehensive MediaWiki pages** covering:
  - **Prologue:** *Utterance of Marvels* (Parts I & II)
  - **Chapter I (Jinzhou Rising):** Acts I–VI (*First Resonance* to *Grand Warstorm*), Interlude (*A New Companion*), Act VII (*Thaw of Eons* across Mt. Firmament), and Act VIII (*To the Shore's End* across Black Shores).
  - **Chapter II (Even When Divinity Remains Silent):** Acts I–XII in Rinascita and Septimont (including *Through the Sea Thou Break*, *The Sacred Breeze So Often Breathes*, *Veils Off in Sun or Shadow*, *What Yesterday Wept, Today Doth Sing*, *Old Man and the Whale*, *The Maiden, The Defier, The Death Crier*, *Shadow of Glory*, *Flames of Heart*, *Dreamcatchers in the Secret Gardens*, *Rust, Sword and the Sun*, *By Sun's Burning Hand*, *By Moon's Fated Light*, *The Bygone Shall Always Return*, *Dawn Breaks on Dark Tides*, *Stagnant Dawn on Wastelands*, and segues).
  - **Chapter III (To the Stars Yet to Shine):** Acts I–V in Lahai-Roi & Roya Frostlands (*When the Unknown Thrums*, *What Burns Beneath Frostlands*, *Ode to the Second Sunrise*, *The Star That Voyages Far*, *Gold Suspended in Shadows*, *Starlights from Yesterdays*, and segues).
  - **Chapter IV (Rebirth From the Depths):** Acts I–IV in Mengzhou (*The Wind Before the Storm*, *Xuanling Sings, Storm Quelled*, *Song of the Heart Sword*, *Simulacrum of the Heart*, and segues).
  - **World & Sentinel Lore:** Sentinel Jué, Sentinel Imperator, Sentinel Hsin, Suoming, Xuanfang Hold, and Mengzhou.
* **Corpus Volume:** Parsed into **3,886 clean, high-signal narrative chunks** (indexed into Qdrant `wuwa_lore`) capturing **1,119 distinct character and speaker tags**.
* **Embedding & Retrieval Enhancements:**
  - Added stop-word downweighting in `generateDeterministicVector` (`internal/rag/embedding.go`) so that high-frequency connector words do not overwhelm domain entities (e.g., Carlotta, Carnevale, Jué, Sentinel, Fractsidus).
  - Filtered out cutscene-only boilerplate markers and isolated non-dialogue fragments (<50 chars) in `internal/parser/parser.go`.
  - Configured `TOP_K=8` and `SCORE_THRESHOLD=0.20` in `.env` and `internal/config/config.go` for broader contextual coverage across lengthy quest narratives.
* **CLI Expansion:** Enhanced `cmd/ingest` with `--all-main-quests`, `--pages-file`, and automated `Makefile` targets (`make gather-all`, `make ingest-all`, `make dry-run-all`).

---

## 3. Directory Layout & File Roles

```text
.
├── cmd/
│   ├── api/
│   │   └── main.go                 # REST API entry point (loads config, starts Gin server)
│   └── ingest/
│       └── main.go                 # Data pipeline CLI (gather, parse, embed, index)
│
├── internal/
│   ├── config/
│   │   └── config.go               # Environment loader (ports, URLs, keys, collection params)
│   ├── fetcher/
│   │   ├── fetcher.go              # MediaWiki API client (parse, search, disk cache)
│   │   └── fetcher_test.go         # Unit tests for HTTP fetcher
│   ├── parser/
│   │   ├── parser.go               # Regex cleaning, template removal, dialogue & chunking logic
│   │   └── parser_test.go          # Unit tests for wikitext parsing and speaker extraction
│   ├── qdrant/
│   │   ├── qdrant.go               # Qdrant REST client (collection setup, upsert, search)
│   │   └── qdrant_test.go          # Unit tests for Qdrant client
│   ├── rag/
│   │   ├── embedding.go            # HuggingFace MiniLM embedder & offline fallback embedder
│   │   ├── llm.go                  # LiteLLM client (4096 token budget, OpenAI format)
│   │   ├── rag.go                  # RAG orchestrator (context formatting, prompt injection)
│   │   └── rag_test.go             # Unit tests for RAG pipeline
│   └── server/
│       ├── server.go               # Gin HTTP server, CORS middleware, handlers
│       └── server_test.go          # Unit tests for HTTP routes and health checks
│
├── data/
│   ├── raw/                        # Raw MediaWiki JSON files (Simulacrum_of_the_Heart, etc.)
│   └── processed/                  # Processed chunks (chunks.json) with speaker metadata
│
├── deploy/
│   ├── docker-compose.yml          # Container configuration for Qdrant, LiteLLM, and API
│   ├── litellm_config.yaml         # Model router configuration (Gemini 3.5 Flash primary)
│   └── .env -> ../.env             # Symlink ensuring compose reads the root .env
│
├── documentation/                  # Visual execution logs and setup screenshots
├── Dockerfile                      # Production multi-stage Alpine build
├── Makefile                        # Common developer commands (build, test, run, ingest)
├── .env.example                    # Sample environment variable template
├── .env                            # Local environment secrets (ignored by Git)
├── .gitignore                      # Git ignore rules
├── go.mod                          # Go module dependencies
├── go.sum                          # Go dependency checksums
├── plan.md                         # Original project requirements document
├── README.md                       # High-level overview & quickstart
└── project-record.md               # This architectural record
```

---

## 4. API Endpoints Reference

Base URL: `http://localhost:8080`

### 1. Health Status
* **Endpoint:** `GET /health`
* **Response:**
  ```json
  {
    "status": "healthy",
    "service": "Wuthering Waves Lore RAG API",
    "qdrant_connected": true,
    "collection": "wuwa_lore",
    "indexed_chunks": 222,
    "timestamp": "2026-10-11T11:53:00Z"
  }
  ```

### 2. Collection Details
* **Endpoint:** `GET /api/v1/status`
* **Response:**
  ```json
  {
    "collection": "wuwa_lore",
    "status": "green",
    "points_count": 222,
    "vectors_count": 222
  }
  ```

### 3. Semantic Search Only (Vector Retrieval)
* **Endpoint:** `POST /api/v1/search`
* **Request:**
  ```json
  {
    "query": "Moon Fox Rice Cakes",
    "limit": 2
  }
  ```
* **Response:** Returns top scored chunks from Qdrant with speaker information, section titles, and relevance score without calling the LLM.

### 4. Full Lore Q&A (RAG Pipeline)
* **Endpoint:** `POST /api/v1/chat`
* **Request:**
  ```json
  {
    "question": "What is the Waking Moon Festival and what role does Sentinel Hsin play?"
  }
  ```
* **Response:**
  ```json
  {
    "question": "What is the Waking Moon Festival and what role does Sentinel Hsin play?",
    "model": "wuwa-lore-model",
    "latency_ms": 6118,
    "total_tokens": 2748,
    "answer": "Based on the lore records from the 'Simulacrum of the Heart' quest line...",
    "sources": [ ... ]
  }
  ```

### 5. On-Demand Live Ingestion
* **Endpoint:** `POST /api/v1/ingest`
* **Request Body (Optional):**
  ```json
  {
    "page": "Simulacrum_of_the_Heart",
    "chunk_size": 900,
    "overlap": 150
  }
  ```

---

## 5. Developer Guide: How to Run & Maintain

### 1. Prerequisites
- Go 1.24+ installed on your system.
- Docker Engine & Docker Compose (or Podman with compose provider).

### 2. Initial Setup
1. Clone the repository and configure your `.env` file:
   ```bash
   cp .env.example .env
   ```
2. Insert your Google Gemini API key:
   ```env
   GEMINI_API_KEY=AIzaSy...
   ```

### 3. Starting the Containers
To launch Qdrant and LiteLLM:
```bash
cd deploy && docker compose up -d
```
Verify they are running:
```bash
docker compose ps
curl http://localhost:6333/readyz      # Should output: all shards are ready
curl http://localhost:4000/health      # Should output: "I'm alive!"
```

### 4. Running Tests
Run all internal unit test suites:
```bash
make test
# Or: go test -v ./...
```

### 5. Data Gathering & Ingestion
* **Gather raw wiki pages without embedding:**
  ```bash
  make gather
  ```
* **Preview chunking without saving to Qdrant (Offline dry run):**
  ```bash
  make dry-run
  ```
* **Full embedding & upsert into Qdrant:**
  ```bash
  make ingest
  ```

### 6. Running the API Server & Web UI Locally
```bash
make run
# The server will start on port 8080
```
Open your browser to:
* **Interactive Lore Q&A (RAG Chat):** `http://localhost:8080/ask.html`
* **Semantic Vector Search:** `http://localhost:8080/search.html`
* **Home / Overview:** `http://localhost:8080/`


---

## 6. Future Expansion Guidelines for New Developers

1. **Adding New Quest Pages:**
   To index a new quest (e.g. `Chapter_IV_Act_II`), simply pass it to the ingestion CLI:
   ```bash
   ./bin/ingest --pages "New_Quest_Title" --save-local
   ```
2. **Switching Embedding Models:**
   The current vector dimension is `384` for `sentence-transformers/all-MiniLM-L6-v2`. If upgrading to a model like `text-embedding-3-small` (1536 dim) or `bge-large-en-v1.5` (1024 dim), update `VECTOR_DIM` in `.env` and recreate the Qdrant collection.
3. **Adding Hybrid Search:**
   Qdrant supports sparse vectors (BM25) alongside dense vectors. You can extend `internal/qdrant/qdrant.go` to include sparse vector payloads for keyword + semantic hybrid search.
4. **Custom Front-end:**
   The Gin server has full CORS enabled (`Access-Control-Allow-Origin: *`), so any React, Vue, or Next.js web interface can communicate directly with `http://localhost:8080/api/v1/chat`.
