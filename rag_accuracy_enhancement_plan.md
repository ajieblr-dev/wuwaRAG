# RAG Accuracy Enhancement Plan

## Objective

This document serves as a checklist for evaluating and implementing **Advanced RAG** techniques within the current codebase. The goal is to resolve the *query-document mismatch* issue where the LLM provides irrelevant answers due to short or insufficient user queries (e.g., searching for specific entities like game lore).

## 🛠️ Phase 1: Current State Analysis (Audit)

* [x] **Chunking Strategy:** Structured objective-aware semantic chunking (`internal/parser/parser.go`). Parses quest scenes by `== Section ==` and `[Objective: ...]` boundaries with a 900-char target and 150-char overlap, filtering cutscene boilerplate and preserving speaker tags and breadcrumb metadata.
* [x] **Embedding Model:** `sentence-transformers/all-MiniLM-L6-v2` (384 dimensions) via HuggingFace Inference API with deterministic character/word n-gram vector hash fallback and stop-word downweighting (`internal/rag/embedding.go`).
* [x] **Vector Database:** Qdrant (running in Docker `:6333` / `:6334`). Stores 3,886 vectors in collection `wuwa_lore`. Supports Cosine distance vector search, multi-field payload filtering, and text scroll matching.
* [x] **Retrieval Logic:** Upgraded from pure vector similarity to **Hybrid Search** (Dense Vector + Exact Lexical Keyword Matching) combined via **Reciprocal Rank Fusion (RRF)**.

## 🚀 Phase 2: Pre-Retrieval (User Input Optimization)

*Focus: Enriching the user's query before sending it to the Vector DB.*

* [x] **1. Implement Query Rewriting / Expansion**
  * **Task:** Create a router or pre-processor function where the LLM restructures the user's input into a richer search query, adding synonyms and context.
  * **Status:** **Completed** (`internal/rag/query_rewriter.go`). Implemented `LLMQueryRewriter` which expands colloquial queries into canonical lore entities (characters, factions, Sentinels, locations) and extracts key search keywords, with a fast heuristic entity extractor fallback (`ExtractHeuristicKeywords`). Configurable via `QUERY_REWRITE_ENABLED=true`.

* [ ] **2. Implement HyDE (Hypothetical Document Embeddings)** (Optional/Experimental)
  * **Task:** Build a pipeline where the LLM generates a temporary answer (hypothesis) based on the initial query, then use that temporary answer to perform the vector search.
  * **Status:** Evaluated as optional fallback. Query Rewriting + Hybrid Search proved more computationally efficient and lore-accurate without hallucination risk.

## 🔎 Phase 3: Retrieval & Chunking Strategy

*Focus: Changing how data is stored and retrieved so specific entities are not lost.*

* [x] **1. Implement Hybrid Search** (High Priority)
  * **Task:** Combine *Vector Search* (for semantics) with *Keyword Search/BM25* (for exact keyword matching like character/place names). Use *Reciprocal Rank Fusion* (RRF) to merge the scores from both search methods.
  * **Status:** **Completed** (`internal/qdrant/qdrant.go`, `internal/rag/hybrid.go`). Implemented Qdrant multi-field text scroll (`ScrollByText`), lexical scoring (`ScorePointLexical`), and standard Reciprocal Rank Fusion (`ReciprocalRankFusion` with $k=60$). Exact entity matches (e.g. *Suhsin*, *Carlotta*, *Averardo Vault*, *Chronosorter*) receive significant ranking boosts. Configurable via `HYBRID_SEARCH_ENABLED=true`.

* [x] **2. Refactor Chunking Strategy: Parent-Child / Scene Window**
  * **Task:** Modify the chunking logic during document ingestion so specific entities are not lost.
  * **Status:** **Completed** (`internal/parser/parser.go`). Chunking splits along quest objectives while preserving scene context headers (`[Objective: ...]`), speakers (`speakers` array), and section titles, eliminating boilerplate cutscene fragments.

## 🎯 Phase 4: Post-Retrieval (Improving Context Quality)

*Focus: Filtering the retrieved documents before passing them to the LLM.*

* [x] **1. Implement Reranker (Cross-Encoder / Composite RRF Reranking)**
  * **Task:** Retrieve a larger top-K result from the Vector DB (e.g., 15-25), then re-sort these results, passing only the top most relevant ones to the LLM for final answer generation.
  * **Status:** **Completed** (`internal/rag/hybrid.go`). The system retrieves a candidate pool of 25 vector results and 25 keyword results, fuses them with RRF, applies composite scoring (RRF rank + vector cosine similarity + entity term density), and filters down to the top `TOP_K` (default 8) highest-relevance evidence snippets. Configurable via `CANDIDATE_POOL_SIZE=25` and `TOP_K=8`.