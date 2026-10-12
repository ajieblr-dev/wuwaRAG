# Skills — curated DESIGN.md references

Vendor of 6 design systems from
[VoltAgent/awesome-design-md](https://github.com/VoltAgent/awesome-design-md.git)
(MIT, extracted public CSS values) for AI-assisted UI work on this repo.

## Contents

| System | Path | Why kept |
|---|---|---|
| Notion | `design-md/notion/DESIGN.md` | Warm minimalism, serif headings — closest to Resonance Archive editorial reader |
| WIRED | `design-md/wired/DESIGN.md` | Paper broadsheet density, reference for long-form lore reading |
| The Verge | `design-md/theverge/DESIGN.md` | Tech editorial, display type + citations rhythm |
| Linear | `design-md/linear.app/DESIGN.md` | Dense dev-tool console pattern for admin surfaces |
| Vercel | `design-md/vercel/DESIGN.md` | Black/white precision, Geist-like tight type for status/health UI |
| Mintlify | `design-md/mintlify/DESIGN.md` | Reading-optimized docs, green accent, search-first |

Project's own system of record stays `web/DESIGN.md` (Resonance Archive).
Files here are references only — do not ship to `web/` unless intentionally adopting.

## Use

1. Copy one into context when prompting an agent, e.g. “build this page to match `skills/design-md/notion/DESIGN.md` while keeping our tokens in `web/DESIGN.md`”.
2. Do not set these as repo-root `DESIGN.md` — agents read root as active system.

## Update

```bash
rm -rf /tmp/opencode/awesome-design-peek
git clone --depth 1 https://github.com/VoltAgent/awesome-design-md.git /tmp/opencode/awesome-design-peek
cp /tmp/opencode/awesome-design-peek/design-md/<name>/DESIGN.md skills/design-md/<name>/DESIGN.md
```
