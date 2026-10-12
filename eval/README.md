# RAG Eval Plan — 5 pages / 222 chunks (no ingestion run)

Scope: `data/raw/*.json` (Simulacrum_of_the_Heart, Hsin, Suoming, Mengzhou,
Xuanfang_Hold) → `data/processed/chunks.json`. Retrieval: `internal/rag/rag.go`
(`TOP_K=5`, `SCORE_THRESHOLD=0.25`, MiniLM-L6-v2 via Qdrant `score_threshold`).
Frontend: `web/app.js` `OD.corpus` / `OD.suggested` (mock-backed prototype).
No Go / web / git-history changes made by this plan.

## 1. What was sampled (read-only)

- `chunks.json` head (Simulacrum indexes 0–44): Steps mega-chunk (index 0,
  6533 chars, speakers polluted — see §4), Dialogue chunks split per
  `[Objective:]` scene (parser `splitByObjectives`), tiny 79–200-char
  labyrinth chunks (indexes 31–44).
- Tail (lines ~4900–5163): Hsin ×3, Suoming ×1, Mengzhou ×5, Xuanfang Hold ×7.
  So ≈206/222 chunks are Simulacrum — entity queries compete against a
  15–200:1 majority page.
- Raw samples: `Hsin.json` / `Suoming.json` (infobox-heavy stubs, mostly
  templates), `Xuanfang_Hold.json` (Intro prose + Areas table),
  `Mengzhou.json` sections list. Parser skips Gallery/Navigation/
  Other-Languages/Change-History; infobox pipes and `{| table` markup
  partially leak into chunk text (see §4).
- `OD.suggested` (4 prompts) maps to eval E01/E02/E04/E05; `OD.corpus` chars
  (168893 / 3167 / 1861 / 5179 / 4028) confirm the skew above.

## 2. Eval set — `eval/eval-set.json` (10 + 1 negative)

Grounded questions with `expected_page`, `must_include` / `must_exclude`,
spoiler flag, and speaker check. Highlights: Moon Fox Rice Cakes (E01),
Suoming org (E02), Vermillion Lock covenant (E03, spoiler), Hsin role (E04),
Xuanfang location (E05), collapse rumor attribution to Suoming (E06, spoiler),
Xuanfang Wardens (E07), Sanctum=Hsin identity (E08, top spoiler), three Heart
Keys multi-chunk (E09), bitter-dislike join (E10), unanswerable Rover control
(E11 — must abstain per `rag.go` rule 2, not hallucinate).
Score per question: expected-page hit in top-K, must-include recall,
must-exclude absence, citation→expected page; suite: page recall@k,
abstention rate, speaker pass rate.

## 3. Versioning scheme for `chunks.json` diffs (design only, not run)

Deterministic IDs today = `sha256(page:title:chunkIndex:first50chars)` → any
re-chunk/re-parse renames IDs, so diff by ID alone is meaningless. Propose a
sidecar manifest (never edit chunks by hand):

- `data/processed/manifest.json`: `{ wiki_fetched_at, page_revisions
  (pageid+revid per page), parser_version (git sha of parser.go),
  targetChunkSize, chunkOverlap, embedding_model, chunk_count_per_page,
  sha256(chunks.json), generated_at }`.
- Diff procedure: keep prior `chunks.json`; on re-ingest, join old↔new on
  `(title, section, subsection, normalized content hash)` — report
  added/removed/modified (content-hash change) + moved (index change only).
- Store manifests per run (`data/processed/manifest-<date>.json`); Qdrant
  upsert is idempotent only if IDs stable — changing chunk size invalidates
  all IDs, so gate re-embeddings on manifest diff, not blind re-ingest.
- Frontend `OD.corpus[].chunks` (currently 0) and mock `indexed_chunks: 222`
  should be generated from the manifest, not hardcoded.

## 4. Threshold tuning notes (design only)

- Baseline `TOP_K=5 / 0.25` is lenient (Qdrant cosine `score_threshold`,
  MiniLM-L6-v2). Risks: tiny labyrinth chunks (E09) score low and crowd out;
  single-chunk pages (Suoming ×1) lose to Simulacrum near-duplicates.
- Proposed sweep without ingestion: for each eval Q, run `SearchOnly` at
  thresholds {0.15, 0.25, 0.35} × topK {5, 8}, record: hit-rate on
  `expected_page`, mean score of expected chunk, #empty results. Expect:
  0.35 → cleaner E09 but drops E02/E04 (short entity chunks); topK 8 helps
  E09/E10 joins, costs prompt tokens (rag.go injects full chunk text).
- Recommendation: keep 0.25 default; add per-query fallback — if top-1 <
  0.35, retry at topK 8 / threshold 0.15 (recall path for entity pages);
  log `score` distribution per page to confirm skew. Do not raise threshold
  until E02/E04 pass at 0.25.

## 5. Spoiler / speaker-attribution checks

- Spoiler: E03/E06/E08 (Sanctum=Hsin, frequency fading, Hold collapse) must
  render behind existing `body[data-spoiler]` gate + carry spoiler flag in
  eval; mock answer path (`OD.mockAnswer`) must never inject real lore.
- Speaker bugs found by sampling (parser.go `ExtractSpeakers`, do-not-fix
  here): (a) Steps index 0 speakers = `["Defeat/Pursue/Approach Suhsin the
  Inevitable"]` — action lines misread as speakers; (b) `???` lines →
  `speakers: null` (indexes 40/42); (c) `Actor No. 1/2` lines uncaptured
  (index 22 keeps only Qingxiao); (d) infobox pipes + `{| table` + `<!--`
  residue leak into Hsin/Suoming/Mengzhou/Xuanfang chunks; (e) `(Rover):`
  parenthesized form vs `Rover:` split. Eval encodes these as `speaker_check`
  fields; any future parser fix must re-run full 11-Q set and diff manifest.
