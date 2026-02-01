# Repository conventions (consistency guide)

This document defines standards for data, paths, and outputs so that all source modules produce uniform, high‑quality artifacts.

## 1) Data schema


## 2) File formats


## 3) File naming


## 4) Directory layout


## 5) Language detection


## 6) Cleaning


## 7) Deduplication


## 8) Logging
- Use simple prints; keep messages short and informative.
- Prefix steps with icons when helpful (optional), e.g., "🔹 Loading", "✅ Done".

## 9) Time and IDs
- Timestamps should be UTC ISO8601 (`YYYY-MM-DDTHH:MM:SSZ`).
- IDs must be stable across reruns given the same inputs; prefer `sha256(text)` possibly combined with deterministic source keys.

## 10) Licenses & provenance
- Always record provenance in `meta` (e.g., `{"url": "...", "license": "..."}`).
- Respect source licenses and robots directives. Do not redistribute restricted data.

## 11) Windows vs. POSIX
- Write code that accepts Windows `\` and POSIX `/` paths.
- In repo docs we show Windows examples; convert as needed for your platform.

## 12) Versioning & changelog
- Update `Docs/CHANGELOG.md` under Unreleased for meaningful changes.
- Tag versions when producing major dataset releases to make comparisons reproducible.
