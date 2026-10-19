<div align="center">

<div style="margin: 20px 0;">
  <img src="./documentation/web-preview-1.png" width="480" alt="WuWaRAG Banner" style="border-radius: 16px; box-shadow: 0 12px 36px rgba(0, 217, 255, 0.25); border: 2px solid rgba(0, 217, 255, 0.4);">
</div>

# 🌊 WuWaRAG: Advanced Wuthering Waves Lore RAG System

<div align="center">
  <img src="https://readme-typing-svg.herokuapp.com?font=Orbitron&size=22&duration=3000&pause=1000&color=00D9FF&center=true&vCenter=true&width=650&lines=Welcome+to+WuWaRAG;Next-Gen+Video+Game+Lore+Intelligence;9%2C470%2B+Vectors+%E2%80%A2+1%2C900%2B+Speakers+%E2%80%A2+1%2C750%2B+Pages;Powered+by+Go%2C+Qdrant%2C+LiteLLM+%26+Gemini" alt="Typing Animation" />
</div>

<div align="center">
  <div style="background: linear-gradient(135deg, #0b132b 0%, #1c2541 50%, #3a506b 100%); border-radius: 15px; padding: 20px; text-align: center; margin: 15px 0;">
    <p>
      <a href='https://github.com/ajieblr-dev/wuwaRAG'><img src='https://img.shields.io/badge/🔥Project-WuWaRAG-00d9ff?style=for-the-badge&logo=github&logoColor=white&labelColor=0f172a'></a>
      <a href='https://golang.org'><img src='https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white&labelColor=0f172a'></a>
      <a href='https://qdrant.tech'><img src='https://img.shields.io/badge/Qdrant-Vector%20DB-DC2626?style=for-the-badge&logo=qdrant&logoColor=white&labelColor=0f172a'></a>
      <a href='https://litellm.ai'><img src='https://img.shields.io/badge/LiteLLM-Proxy%20Gateway-4ecdc4?style=for-the-badge&logo=fastapi&logoColor=white&labelColor=0f172a'></a>
    </p>
    <p>
      <img src="https://img.shields.io/badge/Vectors-9%2C473%20Indexed-00d9ff?style=for-the-badge&labelColor=0f172a">
      <img src="https://img.shields.io/badge/Corpus%20Chunks-9%2C072%20Unified-38ef7d?style=for-the-badge&labelColor=0f172a">
      <img src="https://img.shields.io/badge/Speakers-1%2C928%20Attributed-f5a623?style=for-the-badge&labelColor=0f172a">
      <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white&labelColor=0f172a">
    </p>
  </div>
</div>

</div>

<div align="center">
  <div style="width: 100%; height: 2px; margin: 20px 0; background: linear-gradient(90deg, transparent, #00d9ff, transparent);"></div>
</div>

<div align="center">
  <a href="#-quick-start" style="text-decoration: none;">
    <img src="https://img.shields.io/badge/Quick%20Start-Get%20Started%20Now-00d9ff?style=for-the-badge&logo=rocket&logoColor=white&labelColor=0f172a">
  </a>
  <a href="#-rest-api-endpoints" style="text-decoration: none;">
    <img src="https://img.shields.io/badge/API-Endpoints-38ef7d?style=for-the-badge&logo=postman&logoColor=white&labelColor=0f172a">
  </a>
  <a href="#-admin-console--web-ui" style="text-decoration: none;">
    <img src="https://img.shields.io/badge/Web%20UI-Admin%20Console-f5a623?style=for-the-badge&logo=googlechrome&logoColor=white&labelColor=0f172a">
  </a>
</div>

---

## 🎉 Milestones & News
- [x] **[2026.10] 🚀 Massive Lore Expansion**: Ingested all **CHARACTERS** and **THE WORLD** categories from MediaWiki (1,750+ new pages, 4,762 new chunks). Total vector count scaled to **9,473 vectors** in Qdrant with **1,928 attributed speakers**.
- [x] **[2026.10] ⚡ Parallel High-Throughput Scraper**: Upgraded `cmd/ingest` with multi-worker concurrency pools and deterministic UUID hash deduplication, ensuring **zero duplicate vectors**.
- [x] **[2026.10] 🎯 Hybrid RRF Retrieval with Lexical Dominance**: Implemented Reciprocal Rank Fusion combining 384-d dense embeddings with BM25-style lexical keyword dominance and anti-hallucination query rewriting.
- [x] **[2026.10] 🖥️ Next-Gen Cyberpunk Web & Admin Console**: Built Pen v2-inspired UI with isolated chat sessions, context inspector, and complete Admin Console (`/console`) for corpus explorer, gateway monitoring, and live status.
- [x] **[2026.10] 🤖 Multi-Tier Resilient LLM Gateway**: Powered by LiteLLM Proxy with automatic failover between **Google Gemini 3.1 Flash Lite** (primary) and **Groq LLaMA 3.3 70B** (fallback).

---

## 🌟 System Overview

*Next-Generation Video Game Lore Intelligence*

<div style="background: linear-gradient(135deg, #0b132b 0%, #16213e 50%, #0f3460 100%); border-radius: 15px; padding: 25px; margin: 20px 0; border: 2px solid #00d9ff; box-shadow: 0 0 30px rgba(0, 217, 255, 0.25);">

Video game lore corpora present unique challenges that standard text-only RAG pipelines cannot solve: convoluted multi-speaker dialogue branches, non-linear cutscene scripts, esoteric world terminology, fictional proper nouns (e.g. *Threnodians*, *Sentinels*, *Abraxas*, *Tethys System*), and massive relational entity graphs.

**WuWaRAG** is a production-grade, end-to-end Retrieval-Augmented Generation (RAG) platform tailored for **Wuthering Waves** game lore. It couples a high-throughput Go backend, a multi-stage MediaWiki ingestion pipeline, Qdrant vector database, hybrid Reciprocal Rank Fusion (RRF), and a multi-tier LLM fallback gateway.

With **9,473 indexed vectors**, **9,072 unified chunks**, and **1,928 recognized speakers**, WuWaRAG accurately answers complex player queries, attributes quotes to in-game characters, traces historical timelines across Solaris-3, and prevents LLM hallucinations.

</div>

### 🎯 Key Features

<div style="background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%); border-radius: 15px; padding: 25px; margin: 20px 0; border-left: 5px solid #00d9ff;">

- **🔄 End-to-End Automated Ingestion**: Direct ingestion from Fandom MediaWiki API with wikitext normalization, scene segmentation, and parallel worker pools.
- **🎭 Dialogue & Speaker Attribution**: Automatic parsing of speech tags, narrative headers, dialogue options, and cutscene transitions for 1,928+ characters.
- **🔀 Hybrid Vector & Lexical RRF Search**: Merges dense semantic vector similarity (`sentence-transformers/all-MiniLM-L6-v2`) with weighted keyword lexical search to eliminate proper-noun false negatives.
- **🛡️ Anti-Hallucination Query Rewriting**: Preprocesses user prompts to preserve critical entity tokens (*Abraxas*, *Jué*, *Shorekeeper*, *Camellya*) and prevent semantic drift.
- **⚡ Deterministic Chunk Deduplication**: Every chunk receives a deterministic UUID derived from `pageTitle + section + content`, guaranteeing zero duplicate entries across re-runs.
- **🤖 Resilient Multi-Tier Gateway**: Automatic routing via LiteLLM Proxy from primary **Gemini 3.1 Flash Lite** to **Groq LLaMA 3.3 70B** without client downtime.
- **🖥️ Full Admin Console & Chat Web UI**: Includes Ask Lore view with isolated topic histories, live Corpus Explorer, Gateway Telemetry, and Ingestion controls.

</div>

---

## 🏗️ Architecture & Pipeline

```text
                                  ┌────────────────────────┐
                                  │  Wuthering Waves Wiki  │
                                  │    (MediaWiki API)     │
                                  └───────────┬────────────┘
                                              │ Concurrent Worker Pool
                                              ▼
                                  ┌────────────────────────┐
                                  │   cmd/ingest Pipeline  │
                                  │  Parser & Speaker Map  │
                                  └───────────┬────────────┘
                                              │ Deterministic UUIDs
                                              ▼
┌──────────────────┐              ┌────────────────────────┐
│ Client / Web UI  ├─────────────►│   Go REST API Server   │
│ (Ask / Admin)    │  HTTP / JSON │      (Gin Engine)      │
└──────────────────┘              └───┬────────────────┬───┘
                                      │                │
                          Dense Vector│Hybrid RRF      │Prompt + Context
                                      ▼                ▼
                            ┌─────────────────┐ ┌──────────────────────┐
                            │  Qdrant Vector  │ │    LiteLLM Proxy     │
                            │  (Collection    │ ├──────────┬───────────┤
                            │   wuwa_lore)    │ │ Primary  │ Fallback  │
                            │  9,473 Vectors  │ │  Gemini  │   Groq    │
                            └─────────────────┘ └──────────┴───────────┘
```

<div align="center">
  <div style="width: 100%; max-width: 750px; margin: 20px auto; padding: 20px; background: linear-gradient(135deg, rgba(0, 217, 255, 0.1) 0%, rgba(0, 217, 255, 0.05) 100%); border-radius: 15px; border: 1px solid rgba(0, 217, 255, 0.2);">
    <div style="display: flex; justify-content: space-around; align-items: center; flex-wrap: wrap; gap: 15px;">
      <div style="text-align: center;">
        <div style="font-size: 24px; margin-bottom: 5px;">🌐</div>
        <div style="font-size: 13px; color: #00d9ff; font-weight: bold;">1. Wiki Scraper</div>
      </div>
      <div style="font-size: 18px; color: #00d9ff;">→</div>
      <div style="text-align: center;">
        <div style="font-size: 24px; margin-bottom: 5px;">🧩</div>
        <div style="font-size: 13px; color: #00d9ff; font-weight: bold;">2. Dialogue Parser</div>
      </div>
      <div style="font-size: 18px; color: #00d9ff;">→</div>
      <div style="text-align: center;">
        <div style="font-size: 24px; margin-bottom: 5px;">🔢</div>
        <div style="font-size: 13px; color: #00d9ff; font-weight: bold;">3. Dense Embedder</div>
      </div>
      <div style="font-size: 18px; color: #00d9ff;">→</div>
      <div style="text-align: center;">
        <div style="font-size: 24px; margin-bottom: 5px;">⚡</div>
        <div style="font-size: 13px; color: #00d9ff; font-weight: bold;">4. Hybrid RRF</div>
      </div>
      <div style="font-size: 18px; color: #00d9ff;">→</div>
      <div style="text-align: center;">
        <div style="font-size: 24px; margin-bottom: 5px;">🤖</div>
        <div style="font-size: 13px; color: #00d9ff; font-weight: bold;">5. LLM Synthesis</div>
      </div>
    </div>
  </div>
</div>

### Detailed Stages

#### 1. High-Fidelity Wiki Ingestion
The scraper (`internal/fetcher/fetcher.go`) connects to the Wuthering Waves MediaWiki endpoint (`/api.php?action=parse`), following canonical redirects, resolving page IDs, and stripping MediaWiki boilerplate. It supports multi-worker parallel batch downloading with polite delays.

#### 2. Dialogue & Speaker Attribution Engine
The parser (`internal/parser/parser.go`) analyzes wikitext section by section, identifying dialogue lines (e.g. `Character: dialogue`), objective tags, and scene descriptions. It splits content into ~900 character chunks with 150 character overlaps while keeping speaker lists attached in metadata.

#### 3. Dual Dense & Lexical Hybrid Search (RRF)
Search requests undergo Reciprocal Rank Fusion:
$$\text{RRF Score}(d) = \frac{1}{k + r_{\text{dense}}(d)} + \frac{\alpha}{k + r_{\text{lexical}}(d)}$$
Where $k = 60$ and $\alpha$ provides lexical dominance for exact proper noun matches (such as *Abraxas* or *Jué*).

#### 4. Anti-Hallucination Query Rewriter
Before vector search, `internal/rag/query_rewriter.go` parses the query to extract crucial entity keywords and strips semantic noise. This guarantees that specialized game terminology is preserved in candidate retrieval.

#### 5. Resilient LLM Gateway
Retrieval evidence snippets are formatted into an evidence context block injected into a strict system prompt. The request is routed via LiteLLM to Gemini 3.1 Flash Lite, falling back automatically to Groq LLaMA 3.3 70B if rate limits or quota boundaries are met.

---

## 📊 Knowledge Base & Corpus Statistics

WuWaRAG currently maintains **three unified lore datasets**:

| Dataset Code | Scope & Description | Pages | Chunks | Speakers | Storage Location |
| :--- | :--- | :---: | :---: | :---: | :--- |
| **`main-quest-v1`** | Dialogue scripts from Prologue through Chapter IV + City hubs | 78 | 3,786 | 487 | `data/raw/` |
| **`lore-corpus-v1`** | World Lore, Cosmology, Sentinels, Threnodians, Nations, Timeline | 83 | 537 | 142 | `data/lore_corpus_v1/raw/` |
| **`characters-world-v1`** | Playable Resonators, Backstories, Voicelines, NPCs, Locations, Enemies | 1,750 | 4,762 | 1,441 | `data/characters_world/raw/` |
| **🏆 Unified Corpus** | **Total Knowledge Base indexed in Qdrant (`wuwa_lore`)** | **1,653+** | **9,072** | **1,928** | **`data/processed/chunks.json`** |

---

## 🚀 Quick Start

### Prerequisites
- [Go 1.24+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/engine/install/) or [Podman](https://podman.io/) with Compose support
- [Node.js](https://nodejs.org/) (optional, for running syntax validation)

---

### Step 1: Environment Setup

Clone the repository and copy the environment template:

```bash
git clone https://github.com/ajieblr-dev/wuwaRAG.git
cd wuwaRAG
cp .env.example .env
```

Configure your `.env` file:

```env
# Server
PORT=8080

# Qdrant Vector Database
QDRANT_URL=http://localhost:6333
QDRANT_COLLECTION=wuwa_lore
VECTOR_DIM=384

# LiteLLM Proxy Gateway
LITELLM_URL=http://localhost:4000
LITELLM_KEY=sk-wuwa-rag-secret
LITELLM_MODEL=wuwa-lore-model

# LLM Providers (Fill in your actual keys)
GEMINI_API_KEY=your_gemini_api_key_here
GROQ_API_KEY=your_groq_api_key_here
HUGGINGFACE_API_KEY=
```

*(Note: If Hugging Face API keys are omitted, WuWaRAG automatically uses a local deterministic semantic embedding fallback, ensuring 100% offline development functionality).*

---

### Step 2: Start Infrastructure (Docker / Podman)

Launch Qdrant and LiteLLM services in the background:

```bash
cd deploy
docker compose up -d
# or: podman compose up -d
```

Verify services are running:
```bash
docker compose ps
# Qdrant is available on http://localhost:6333
# LiteLLM is available on http://localhost:4000
```

---

### Step 3: Run Ingestion (Optional if using pre-built chunks)

To re-index or ingest lore pages from MediaWiki:

```bash
# Preview parsing without writing to Qdrant (Dry Run)
make dry-run

# Ingest characters and world lore from disk cache
go run ./cmd/ingest \
  --pages-file data/characters_world/pages.txt \
  --raw-dir data/characters_world/raw \
  --dataset-code "characters-world-v1" \
  --from-cache \
  --out data/characters_world/chunks.json

# Live parallel fetch from MediaWiki with 8 workers
go run ./cmd/ingest \
  --pages-file data/characters_world/pages.txt \
  --raw-dir data/characters_world/raw \
  --gather-only \
  --concurrency 8
```

---

### Step 4: Run the API Server

```bash
make run
# Or: go run ./cmd/api/main.go
```

The server starts at `http://localhost:8080`.

---

## 🌐 REST API Endpoints

### 1. Health Status
```http
GET /health
```
```json
{
  "status": "healthy",
  "service": "Wuthering Waves Lore RAG API",
  "collection": "wuwa_lore",
  "indexed_chunks": 9473,
  "qdrant_connected": true,
  "litellm_connected": true,
  "timestamp": "2026-10-19T06:06:41Z"
}
```

### 2. Lore Q&A (RAG Chat)
```http
POST /api/v1/chat
Content-Type: application/json

{
  "question": "Who is Shorekeeper in Wuthering Waves?"
}
```
```json
{
  "question": "Who is Shorekeeper in Wuthering Waves?",
  "answer": "The Shorekeeper, known as the 'Second Instance', is the mythical Guardian of the Black Shores. She was created from high-purity Remnant Energy and Sonoro Spheres to serve as the core of the Tethys System, dedicated to protecting the Rover and Solaris-3.",
  "sources": [
    {
      "id": "1f07adde-a9ee-9333-1b65-79faae527c7b",
      "dataset_code": "main-quest-v1",
      "title": "Advance toward the Future from Today",
      "section": "Dialogue",
      "speakers": ["The Shorekeeper"],
      "content": "The Shorekeeper: When you established the Black Shores to unite Solaris, the Tethys System already existed...",
      "score": 0.83
    }
  ],
  "model": "wuwa-lore-model",
  "latency_ms": 380,
  "total_tokens": 312
}
```

### 3. Hybrid Semantic Search
```http
POST /api/v1/search
Content-Type: application/json

{
  "query": "Abraxas",
  "top_k": 5
}
```

### 4. Corpus Data & Stats
```http
GET /api/v1/corpus?page=Camellya&limit=10
GET /api/v1/status
GET /api/v1/speakers
```

---

## 🖥️ Admin Console & Web UI

WuWaRAG includes an integrated web client and administration console:

- **Ask Lore Interface (`/ask.html` or `/`)**: Cyberpunk chat interface featuring isolated chat sessions, quick topic switcher, context panel displaying retrieved evidence snippets, and session clearing confirmation modals.
- **Corpus Explorer (`/admin-corpus.html` or `/console`)**: Full-text search and inspector across all 9,072 chunks, speaker analytics, and page breakdown.
- **Gateway Monitor (`/admin-gateway.html`)**: Real-time telemetry, model routing, latency, and token counters.
- **Ingestion Portal (`/admin-ingest.html`)**: Live web controls for running batch ingestions and monitoring pipeline jobs.
- **Design System (`/design-system.html`)**: Interactive catalog of UI components, color palettes, and Tacet Resonance tokens.

---

## 🧪 Testing & Verification

Run the full Go test suite:

```bash
make test
```

Run frontend smoke and syntax validation:

```bash
./scripts/smoke-web.sh
```

All test suites verify:
- Wikitext template parsing, character dialogue tag normalization, and speaker attribution.
- Qdrant REST interaction (collection creation, batch upserting, and hybrid search).
- Lexical dominance, heuristic keyword extraction, and reciprocal rank fusion ranking.
- LLM prompt context injection and LiteLLM failover behavior.
- Static file routing and session isolation in `/ask.html`.

---

## 📁 Repository Structure

```text
.
├── cmd/
│   ├── api/                 # Main entry point for REST API server
│   │   └── main.go
│   └── ingest/              # High-throughput ingestion & chunking CLI
│       └── main.go
├── data/
│   ├── main_quests.txt      # 78 Main Quest page titles
│   ├── raw/                 # Raw JSON dumps for main quests
│   ├── lore_corpus_v1/      # 83 World lore pages, raw files, manifest
│   ├── characters_world/    # 1,750 Character & World pages, raw files, manifest
│   └── processed/
│       └── chunks.json      # Master unified corpus (9,072 chunks)
├── internal/
│   ├── config/              # Configuration and .env loader
│   ├── fetcher/             # MediaWiki API client with rate limiting
│   ├── parser/              # Wikitext cleaner, dialogue extractor & chunker
│   ├── qdrant/              # Qdrant vector database REST client
│   ├── rag/                 # RAG engine, hybrid RRF, query rewriter, LLM client
│   └── server/              # Gin HTTP handlers, routes, and admin endpoints
├── deploy/
│   ├── docker-compose.yml   # Multi-service stack (qdrant, litellm, api)
│   └── litellm_config.yaml  # Multi-tier routing (Gemini + Groq fallback)
├── web/                     # Classic & Admin Console views
├── web_redesign/            # Reskinned cyberpunk web interface
├── scripts/
│   └── smoke-web.sh         # Frontend validation script
├── Dockerfile               # Production multi-stage Docker build
├── Makefile                 # Development shortcuts (build, test, run, ingest)
└── .env.example             # Configuration template
```

---

## 📜 License & Acknowledgments

- **Game Content & Lore**: Wuthering Waves is developed and published by [Kuro Games](https://wutheringwaves.kurogames.com/). All game text, characters, and assets belong to their respective copyright holders.
- **Data Source**: Community-curated game lore from the [Wuthering Waves Fandom Wiki](https://wutheringwaves.fandom.com/).
- **Vector Database**: Powered by [Qdrant](https://qdrant.tech/).
- **LLM Gateway**: Built with [LiteLLM](https://litellm.ai/).
- **Reference Inspiration**: Project structure and documentation aesthetic inspired by [hkuds/rag-anything](https://github.com/hkuds/rag-anything).
