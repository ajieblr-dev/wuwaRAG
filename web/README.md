# Resonance Archive — Reader + Admin Prototype (web/)

Static mock-backed prototype for the Wuwa lore RAG service.
Unofficial fan project, not affiliated with Kuro Games.

## Screens

- `index.html` — landing, worked example, route map
- `ask.html` — reader: Q&A with footnote citations (default surface)
- `search.html` — raw semantic search, no LLM
- `about.html` — scope, pipeline, attribution
- `admin.html` — admin gate (token)
- `admin-overview.html` — health, KPIs, topology
- `admin-ingest.html` — re-ingestion form + CLI mirror
- `admin-corpus.html` — pages / speakers / chunk inspector (mock)
- `admin-gateway.html` — routing chain, latency/token stats, probe
- `admin-logs.html` — client-side request log
- `admin-settings.html` — base URL, token, theme, density, spoilers
- `design-system.html` — tokens, components, states (light + dark)

Shared runtime: `app.js` (settings, mock corpus, API client, shell, palette).
Styling: `styles.css`. Fonts self-hosted in `fonts/` (Inter, Newsreader, JetBrains Mono).

## Run

No build. Serve statically:

```bash
# from repo root
python3 -m http.server 8000 --directory web
# open http://localhost:8000/ask.html
```

Or with any static server (`npx serve web`, `caddy`, nginx).

## Backend contract

Expects API base `http://localhost:8080` (editable in Settings):

- `GET /health`
- `GET /api/v1/status`
- `POST /api/v1/search` `{query, limit}`
- `POST /api/v1/chat` `{question}` → `{question, model, latency_ms, total_tokens, answer, sources[]}`

Default is **MOCK mode** (`od-mock=true` in localStorage) — no backend required.
Toggle MOCK off in Settings to hit the live Go API.

Auth: admin token entered at runtime, held in `sessionStorage` (`od-token`),
sent as `Authorization: Bearer <token>`. Never in `localStorage`, never bundled.

## Design docs

- `DESIGN.md` — visual system, tokens, layout, motion, voice
- `DESIGN-HANDOFF.md` + `DESIGN-MANIFEST.json` — implementation handoff
- `opendesign-wuwa-lore-rag-prompt.md` — source prompt spec
- `context/source-context.md` — provenance
