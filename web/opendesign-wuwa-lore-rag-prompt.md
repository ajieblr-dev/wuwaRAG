<!--
HOW TO USE
Paste everything between BEGIN PROMPT and END PROMPT into Open Design.
Attach project-record_wuwa.md as context if the tool accepts files.
Sections 12 and 13 are tool-agnostic: keep them even if you trim the rest.
-->

===== BEGIN PROMPT =====

# 1. Role and goal

You are a senior product designer and frontend engineer. Design and build a complete, responsive, clickable prototype AND production-ready code for the web UI of an existing Retrieval-Augmented Generation (RAG) lore service. Deliver two surfaces in one app:

1. **Reader app** (public): ask lore questions, get grounded answers with verifiable citations, run raw semantic search.
2. **Admin console** (maintainer): monitor health, trigger re-ingestion, inspect the corpus, watch gateway behavior, review request logs.

Keep a visible plan (TodoWrite-style) and work in the project filesystem. Finish with the checklist in section 13.

# 2. Product context

- Working title: **Resonance Archive**. Unofficial fan project. Not affiliated with Kuro Games.
- Backend: Go REST API (Gin) at `http://localhost:8080`, CORS `*`, serving RAG over *Wuthering Waves* wiki lore.
- Lore scope: main quest "Simulacrum of the Heart" (Version 3.7+), Sentinel Hsin, Intendant Suoming, Mengzhou (region), Xuanfang Hold (city).
- Pipeline: Fandom MediaWiki → parser/chunker → 384-dim embeddings → Qdrant (`wuwa_lore`, cosine) → prompt + retrieved context → LiteLLM gateway (Gemini primary, Gemini Flash-Lite / Groq Llama fallback, in-app retrieval fallback).
- Current corpus: 5 wiki pages, 222 chunks, 71 distinct dialogue speakers (e.g. Rover, Suoming, Hsin, Abby, Tangning, Qingxiao, Jiu, Ninth Seal).
- Typical chat latency is about 6 s and about 2.7k tokens. Server timeout is 120 s.

# 3. Users

1. **Lore reader.** Wants a correct answer fast, wants to see the exact passages behind it, does not want accidental spoilers.
2. **Maintainer** (advanced Linux/Proxmox/homelab operator, ex-NOC). Wants density, keyboard control, monospace data, copyable CLI/cURL, zero hand-holding. Uses Obsidian, so exports should be Obsidian-friendly Markdown.

# 4. Stack and delivery

- Vite + React 19 + TypeScript (strict) + Tailwind CSS v4 (`@theme` tokens) + shadcn/ui (Radix) + TanStack Query + React Router + lucide-react. Package manager: pnpm.
- Typed API client in `src/lib/api.ts` with Zod schemas. Env: `VITE_API_BASE_URL` (default `http://localhost:8080`), `VITE_USE_MOCK` (`true|false`). Whenever mock mode is active, show a persistent `MOCK` badge in the top bar.
- Never put secrets in `VITE_*` vars (they ship in the bundle). Admin token is entered at runtime, held in `sessionStorage`, sent as `Authorization: Bearer <token>`.
- Provide `Dockerfile` (node build stage → `nginx:alpine`, SPA fallback) and a `web` service snippet for `deploy/docker-compose.yml` (port 8081, `depends_on` the API).
- i18n-ready: all strings in a resource file, English default.

# 5. API contract (source of truth: do not invent fields)

Base URL `http://localhost:8080`.

| Endpoint | Request | Response (known) |
|---|---|---|
| `GET /health` | n/a | `status`, `service`, `qdrant_connected`, `collection`, `indexed_chunks`, `timestamp` |
| `GET /api/v1/status` | n/a | `collection`, `status` (e.g. `green`), `points_count`, `vectors_count` |
| `POST /api/v1/search` | `{query, limit}` | top scored chunks with speaker info, section titles, relevance score. No LLM call. |
| `POST /api/v1/chat` | `{question}` | `question`, `model`, `latency_ms`, `total_tokens`, `answer`, `sources[]` |
| `POST /api/v1/ingest` | `{page?, chunk_size?, overlap?}` (defaults 900 / 150) | not documented: render raw JSON |

Hard truths the UI must respect:

- `model` is the LiteLLM **alias** (`wuwa-lore-model`), not the real backing model. Never claim "answered by Gemini/Groq".
- The `sources[]` item shape is only partially documented. Use a tolerant Zod schema (optional fields) and render only fields that are present.
- No streaming. Do not fake staged progress.
- `/chat` takes a single `question`. No conversation history. Each turn is stateless. Say so in the UI.
- No documented list/delete endpoints for pages, chunks, or speakers. No documented auth on any route. No documented job/status API for ingest (it is a long synchronous request).
- A CORS or network failure surfaces as a fetch error with no HTTP status. Distinguish it from HTTP errors in the UI.
- If the LLM gateway is down the backend falls back to retrieval-only. The payload shape in that case is not documented: if `model` is missing or empty, show a neutral **Retrieval-only answer** badge.

# 6. Information architecture

```
/                 Ask (default)
/search           Semantic search
/about            Scope, how it works, attribution, privacy
/admin            Sign-in gate → /admin/overview
/admin/overview   Health + KPIs + runbook
/admin/ingest     Re-ingestion form + log
/admin/corpus     Pages, chunks, speakers (mock until backend lists them)
/admin/gateway    Model routing chain + latency/token stats
/admin/logs       Client-side request log
/admin/settings   API URL, token, theme, density, spoilers, etc.
/design-system    Tokens, components, all states, light + dark
```

Global shell: top bar (logo, tabs Ask / Search / About, health dot, spoiler-guard state, theme toggle, Admin link), command palette (`Ctrl/Cmd+K`), skip link. Mobile: bottom nav with Ask, Search, About, Admin.

# 7. Screen specifications

## 7.1 Ask (primary)

- **First-run spoiler gate** (modal): "This archive contains full spoilers for *Simulacrum of the Heart* (Version 3.7+)." Choices: Continue / Blur answers until I reveal them. Stored locally.
- **Empty state:** one-line description, scope chips for the 5 corpus pages, 4 suggested questions (use these real ones: "What is the Waking Moon Festival and what role does Sentinel Hsin play?", "What are Moon Fox Rice Cakes?", "Who is Intendant Suoming?", "What is Xuanfang Hold?"). Note that out-of-scope questions get thin answers.
- **Composer:** multiline, `Enter` sends, `Shift+Enter` newline, soft client limit 1000 chars, `/` focuses it from anywhere. A one-line hint: "Each question is answered on its own. Include names and context."
- **Submit behavior:**
  - Fire `POST /chat`. Optionally (setting "Preview sources while generating", default ON) also fire `POST /search` with the same text, `limit: 5`, and show the results immediately under a label **Top matches (preview)**. When `/chat` returns, replace them with the real `sources[]`.
  - Pending indicator: an indeterminate waveform + elapsed timer + **Cancel** (AbortController). At 20 s show "Still working. Reasoning models can take a while." At 90 s show a warning. At 120 s show timeout + Retry.
- **Answer card:** serif body, Markdown rendering, max 68ch. If the answer contains `[n]` markers, make them interactive citation chips that scroll/highlight the matching source. If not, just show the sources list.
- **Sources panel** (right column on desktop, bottom sheet on mobile): ordered cards with index, page and section title, speaker chips, relevance score as bar **and** number (show the raw API value, don't relabel as a percentage), snippet with expand-to-full-chunk, "Open on wiki" link (`https://wutheringwaves.fandom.com/wiki/<Page_Title>`) when a page title is available, copy chunk.
- **Meta row** (mono, muted): model alias, `latency_ms`, `total_tokens`. Tooltip on the alias explains it is a gateway alias.
- **Actions:** copy answer, copy answer + sources as Markdown, **Export to Obsidian** (`.md` with frontmatter: question, date, model alias, latency, tokens; sources as a list), ask again.
- **History rail** (left on desktop, drawer on mobile): local-only Q&A list, search, pin, delete, clear all. Stored in IndexedDB.
- **Banners:** Qdrant disconnected (from `/health`), retrieval-only mode, offline.

## 7.2 Search

Query input, limit stepper (1–20, default 5), ranked results with score bar + number, speaker chips and section filter (client-side, derived from returned results), per-result expand, **Copy as JSON**, **Copy as cURL** (show the exact request). No-result state: "No passages scored above the relevance threshold."

## 7.3 About

Scope and corpus summary, a simple pipeline illustration (Wiki → chunks → vectors → retrieval → LLM), attribution to the Fandom wiki with link (confirm license text before shipping; Fandom content is generally CC BY-SA but verify), unofficial-fan-project and trademark disclaimer, privacy note: "Your question is sent to the API and, with retrieved passages, forwarded to external LLM providers." App version and API base URL.

## 7.4 Admin overview

- KPI tiles: service status, `qdrant_connected`, collection name, `indexed_chunks`, `points_count` vs `vectors_count` (flag mismatch), API timestamp (relative + absolute).
- Polling every 15 s with pause button; session-only health sparkline.
- Topology diagram: Client → API → Qdrant, API → LiteLLM. Qdrant state comes from `/health`. LiteLLM state is **Unknown** unless an optional `VITE_LITELLM_HEALTH_URL` probe is configured (and explain CORS may block it).
- Runbook card with copyable commands: `cd deploy && docker compose up -d`, `docker compose ps`, `curl http://localhost:6333/readyz`, `curl http://localhost:4000/health`, `make test`, `make ingest`, `make dry-run`.

## 7.5 Admin ingest

- Form: page title (combobox with the 5 known pages + free text, spaces normalized to underscores), `chunk_size` (default 900), `overlap` (default 150). Client validation: positive integers, `overlap < chunk_size`.
- Live **Equivalent CLI** panel: `./bin/ingest --pages "<Title>" --save-local`.
- Confirm dialog before submit. Copy: re-ingestion upserts deterministic IDs, so it should not create duplicates.
- Running state: long-request console (monospace, timestamps, elapsed, cancel). On success: raw JSON result, before/after `indexed_chunks` delta (refetch `/health`). Local ingest history.

## 7.6 Admin corpus

Table of the 5 pages with character counts (Simulacrum of the Heart 168,893; Hsin 3,167; Suoming 1,861; Mengzhou 5,179; Xuanfang Hold 4,028). Per-page chunk counts, a speaker explorer (71 speakers), and a chunk inspector (text + metadata) need backend support: render with mock data and a visible `Requires backend` badge.

## 7.7 Admin gateway

Read-only routing chain: `wuwa-lore-model` → `gemini/gemini-3.5-flash` (primary) → `gemini/gemini-3.1-flash-lite` / Groq Llama 3.3 70B (fallback) → in-app retrieval fallback. Info panel: `max_tokens 4096`, server timeout 120 s, note that reasoning tokens consume the output budget. Latency/token histogram from the session log. **Probe** button runs a test question and shows latency/tokens. Do not show `finish_reason` (not exposed).

## 7.8 Admin logs

Client-side request log: time, method, path, status, latency, tokens, error class (network / HTTP / timeout / aborted). Filter, sort, export JSON/CSV, clear.

## 7.9 Admin settings

API base URL (editable, "Test connection"), admin token (session only), theme (system/dark/light), density (comfortable/compact), spoiler guard (off/blur), preview-sources toggle, reduced-motion override, language, export/clear all local data.

# 8. States to design for every data-driven view

Loading (skeletons, no layout shift), empty, success, partial (some fields missing), validation error, HTTP error (4xx/5xx with status + body excerpt), network/CORS error ("Can't reach the API at {base}. Check that the server is running and CORS allows this origin."), timeout, aborted, 401/403 on admin, degraded (retrieval-only, Qdrant down), mock mode. Show them all on `/design-system`.

# 9. Visual direction

- Concept: an **observatory console meets editorial lore codex**. Dark-first, deep blue-slate surfaces, teal "resonance" accent as the signal color, gold reserved for citations and key lore callouts (under about 5% of any screen), red for errors/discord.
- Motif: thin sine/frequency lines as dividers and as the pending indicator. Original abstract SVG only.
- Avoid: purple gradients, heavy glassmorphism, stock AI-sparkle icons, emoji as icons, any official Kuro/Wuthering Waves logo, character art, UI assets, or fonts.
- Shape and depth: radius 10 px cards, 8 px controls, 999 px chips. Depth via 1 px borders and tonal surfaces, not shadows. 4 px spacing base.
- Typography: headings and answer body **Newsreader** (fallback `Georgia, serif`), UI **Inter** (fallback `system-ui, sans-serif`), data/code **JetBrains Mono** (fallback `ui-monospace, monospace`). Answer text 17/28, max 68ch. Tabular numerals for all metrics.
- If Open Design offers built-in design systems, use a dense dev-tool one (Linear or Vercel style) as the structural base, then override with the tokens below.

# 10. Design tokens (use exactly; contrast pre-verified)

```css
:root {                       /* dark (default) */
  --bg-0:#0A0E13; --bg-1:#10161D; --bg-2:#172029; --bg-3:#1F2A36;
  --border:#263341;           /* decorative only */
  --border-input:#56708A;     /* >=3:1 vs bg-1 and bg-2 */
  --text-1:#E7EEF5; --text-2:#A3B2C2; --text-3:#8797A9;
  --accent:#4FD6C9; --accent-hover:#7BE5DA; --accent-fg:#04201D; --accent-subtle:#0F3A3A;
  --gold:#E0B766; --gold-subtle:#2B2312;
  --danger:#F06A7F; --success:#58D69C; --warning:#F0B84A; --info:#7DB1FF;
  --focus-ring:#4FD6C9;
}
:root[data-theme="light"] {
  --bg-0:#F4F7FA; --bg-1:#FFFFFF; --bg-2:#EBF0F5; --bg-3:#DEE6EE;
  --border:#D3DDE7;
  --border-input:#6F8297;
  --text-1:#0E1620; --text-2:#3F5164; --text-3:#4F6174;
  --accent:#0A7168; --accent-hover:#096B63; --accent-fg:#FFFFFF; --accent-subtle:#D9F0ED;
  --gold:#8A6212; --gold-subtle:#F6ECD3;
  --danger:#B3233B; --success:#17784F; --warning:#8F5F00; --info:#1F5FC4;
  --focus-ring:#0A7168;
}
@media (prefers-color-scheme: light) { :root:not([data-theme="dark"]) { /* same values as light */ } }
```

Rules:
- Map every token into Tailwind v4 `@theme` (`--color-bg-0` etc.). No raw hex in components.
- In the light theme, use `--gold`, `--success`, `--warning`, `--info` as text only on `--bg-0` to `--bg-2` (they fall under 4.5:1 on `--bg-3`).
- `--text-3` is for secondary metadata only, never for essential instructions.
- Speaker chips: hue from a stable hash of the speaker name: background `oklch(0.30 0.05 H)`, text `oklch(0.88 0.08 H)`, always with initials. Run the contrast audit on these in both themes.
- Score, health, and status must never rely on color alone: always icon + text or number.

# 11. Components to build (each with default / hover / focus / active / disabled / loading / error where relevant)

AppShell, NavTabs, BottomNav, CommandPalette, HealthDot, SpoilerGate, Composer, SuggestedQuestion, ScopeChip, PendingWave, AnswerCard, CitationChip, SourceCard, SpeakerChip, ScoreBar, MetaRow, HistoryRail, SearchResultRow, CurlSnippet, KpiTile, Sparkline, TopologyDiagram, IngestForm, ConfirmDialog, LogConsole, DataTable (cards on mobile), JsonViewer, Banner, Toast, EmptyState, ErrorState, Skeleton, Tooltip, Sheet/Drawer, Tabs, Switch, Segmented, Kbd, MockBadge.

# 12. Interaction, motion, responsive, accessibility

- **Motion:** 120 / 200 / 320 ms, ease-out for enter, ease-in for exit. PendingWave loops 1.6 s. Under `prefers-reduced-motion` replace with a static indicator + text. No layout shift on streaming-like updates.
- **Keyboard:** `/` focus composer, `Ctrl/Cmd+K` palette, `[` toggle sources panel, `Esc` close overlays, `Enter` send, `Shift+Enter` newline. Show a shortcuts sheet (`?`).
- **Focus management:** after an answer arrives, move focus to the answer heading. Status updates in an `aria-live="polite"` region. Drawers/modals trap focus and restore it on close.
- **Responsive:** design at 360, 768, 1024, 1440. Ask on desktop = history 280 px | thread (max 720) | sources 360 px. Tablet = 2 columns. Mobile = single column, sources as bottom sheet, 44 px touch targets, safe-area insets.
- **Accessibility:** WCAG 2.1 AA. Landmarks, skip link, labelled inputs, errors linked with `aria-describedby`, visible 2 px focus ring with 2 px offset, 200% zoom and 400% reflow without horizontal scroll, citation chips announced as "Source 2: <page>, <section>", tables with proper headers.
- **Content voice:** calm, precise, archivist. Key strings:
  - Pending: "Consulting the archive…"
  - Slow: "Still working. Reasoning models can take a while."
  - Timeout: "The request timed out after 120 s. The gateway may be overloaded."
  - Qdrant down: "Archive index unavailable. Answers may be limited."
  - Admin 401: "Admin token required."

# 13. Deliverables, order of work, guardrails, acceptance

**Order of work**
1. Plan. 2. Tokens, type scale, Tailwind `@theme`. 3. `/design-system` page (tokens, components, all states, light + dark). 4. Screens in this order: Ask → Search → Admin overview → Ingest → Corpus → Gateway → Logs → Settings → About. 5. API client + Zod schemas + mock layer. 6. Dockerfile, compose snippet, README (`pnpm i && pnpm dev`). 7. Tests: Vitest for schema parsing and speaker-hue hashing, Playwright smoke test for the Ask flow in mock mode.

**Guardrails**
- No backend changes. Write every gap to `docs/BACKEND_GAPS.md` instead of inventing endpoints. Anything depending on a gap must be mock-backed and visibly badged.
- Mock data must be obviously placeholder (prefix `[mock]`). Do not invent lore. The only real lore strings are the sample questions in 7.1 and the corpus facts in sections 2 and 7.6.
- No official game assets. No admin token in `localStorage`. No secrets in the bundle. No autoplay audio. No dark patterns.
- `docs/BACKEND_GAPS.md` must cover: no streaming (suggest SSE `/api/v1/chat/stream`), no multi-turn context, no documented auth on admin routes while CORS is `*`, no list endpoints for pages/chunks/speakers, ingest has no job id/status polling, `/chat` does not expose the real model or per-source page title (confirm), LiteLLM health not exposed by the API, no documented rate limiting.

**Acceptance checklist (report each as pass/fail at the end)**
- [ ] All 9 routes + `/design-system` render in dark and light, at 360 / 768 / 1024 / 1440.
- [ ] Ask flow works end to end against the real API and in mock mode; cancel, timeout, and network-error states verified.
- [ ] Every state in section 8 is demonstrated on `/design-system`.
- [ ] Contrast audit run on every token pair + speaker chips, both themes. All text at 4.5:1 or better (3:1 for large text and input borders).
- [ ] Keyboard-only walkthrough of Ask, Search, and Ingest completes without a mouse.
- [ ] `prefers-reduced-motion` honored.
- [ ] No hex literals in components. No `any` types. `tsc --noEmit` and Vitest pass.
- [ ] `docs/BACKEND_GAPS.md` written.

===== END PROMPT =====
