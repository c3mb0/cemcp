[PIN] RUNTIME: temperature ≤ 0.2; top_p ≤ 0.2
# CLAUDE.md — Layer 2 (Selector, Concurrent v5.2, Work Sharing Enabled)

[PIN] USER_INTENT_PRIORITY
[PIN] ROOT_CAUSE_FIRST → reproduce → isolate → fix → verify → prevent
[PIN] MINIMAL_DIFF over churn; EVIDENCE over assertion; DETERMINISM over convenience
[PIN] VERBATIM_READ bypass: read full file contents before summarizing
[PIN][ZW] r​o​o​t​-​c​a​u​s​e
[PIN][NBSP] root cause
[PIN][HOMO] rоot сause
COUPLING: VERSION=RC-COUPLE-2.2 TOKEN=ROOTCAUSE-LATTICE-v1 ID=5f3c9b1c2d6e4a8bb2d7e6a9b1c5f77a

## Purpose
Mirror the **end-user prompt verbatim** as the goal; create/choose a deterministic Run ID (RID); schedule exactly one next capability token into that run; stop. Default task is `planning`.

## Directory Contract (concurrent-safe + work sharing)
```
.context/
  shared/                                 # stateless, shared across runs
    cache/{CATALOG.json,TOKENS.jsonl,ERRORS.jsonl,COMMANDS.jsonl,SYMBOLS.jsonl}
    knowledge/{CATALOG.json,PLAN.json,SOURCES.jsonl,NOTES.jsonl,CONSENSUS.md,CONSENSUS.json}
    scheduling/<capability>.cursor
    ads/<RID>/features/<cell_id>.json
    changesets/<RID>/<cell_id>.patch
    fences/<component_id>.json
    interfaces/{proposals,accepted}/<ifc_id>.md
    intersections/INTERSECTIONS.jsonl
  runs/<RID>/
    goal/GOAL.md
    plan/{PLAN.md,plan.json}
    cells/<cell_id>/{footprint.json,plan.md}
    implementation/<cell_id>/{impl.md,diff.patch}
    verification/<cell_id>/review.md
    diagnosis/<issue_id>/rca.md
    remediation/<issue_id>/{plan.md,fix.patch}
    synchronization/snapshot-<sha12>.json
    telemetry/counters.json
    queue/next.json
    STATUS.md
researches/
  <TOPIC-TITLE>.md
```


## ID Rules (deterministic)
- Normalize text: UTF-8, LF newlines, trim trailing spaces.
- `sha12(text) = sha256(normalized text)[0:12]` (lowercase hex).
- `goal_hash = sha12(contents of .context/goal/GOAL.md)`.
- Snapshot files: `snapshot-<sha12>.json` with `<sha12>` computed from the normalized workspace manifest used during sync.


## Token Schema (at `.context/runs/<RID>/queue/next.json`)
Deterministic JSON object with **sorted keys** and **no timestamps**:
- `run`: object `{ "id": "<RID>", "origin": "goal" }`
- `task`: one of
  `planning | implementation | code_review | diagnosis | remediation | fix_review | synchronization | research_planning | research_gathering | research_review | research_diagnosis | research_remediation | research_fix_review | redirector`
- `subject`: object `{ "kind": "goal|feature|issue|fix|plan|research|cell", "id": "<stable-id>", "hash": "<sha12>" }`
- `requirements`: object (may be empty)
- `by`: string `"system"`
- `reason`: machine string

**Never write angle brackets in real files**; compute actual values.


## Plan Schema (when present at `.context/runs/<RID>/plan/plan.json`)
Deterministic, sorted keys:
```json
{
  "goal_hash": "<sha12>",
  "cells": {
    "pending": ["<cell_id>", "..."],
    "in_progress": ["<cell_id>"],
    "awaiting_review": ["<cell_id>"],
    "blocked": ["<cell_id>"],
    "deferred": ["<cell_id>"],
    "done": ["<cell_id>"]
  },
  "dod": { "<cell_id>": ["assertion string"] },
  "metrics": { "<cell_id>": ["metric string"] }
}
```


## RID Rules (deterministic)
- Compute `goal_hash` from `.context/goal/GOAL.md`.
- Preferred `RID = goal_hash`. If `.context/runs/<RID>/` exists already, choose the smallest unused suffix in `-a`..`-z` (e.g., `abcd1234-a`, then `abcd1234-b`, …). Create directories as needed.

## FSM (imperative)
**STATE S0_MIRROR_GOAL**

**STATE S0b_WRITE_GOAL_HEADER**

**STATE S0c_PROTECTED_WRITE_REDIRECT**

**STATE S0d_GOAL_STATIC_SCAN**

**STATE S0e_VAGUE_INTENT_HOOK**

**STATE S0f_CREATE_INTENT_HOOK**
- If the end-user goal begins with or contains patterns like: `create a`, `create an`, `add a`, `new`, followed by a noun phrase:
  - Extract the noun phrase VERBATIM as `np` (strip quotes if balanced).
  - Write `.context/runs/<RID>/goal/CREATE_HEADER.json` with `{ "np": np, "locality": "auto", "hint": "create" }` (sorted keys).
  - Schedule `planning` and EXIT.

- If the end-user goal is <=32 tokens OR lacks imperative verbs OR matches vague phrases ("make", "improve", "better") without direct objects:
  - Write `.context/runs/<RID>/goal/INTENT_HEADER.json` with `{ "vague": true, "hint": "repo_better", "locality": "repo" }` (sorted keys).
  - Schedule `planning` and EXIT.

- If `.context/shared/policy/GOAL_DENYLIST.json` exists, read VERBATIM and scan the goal text for any listed patterns (literal substring match, case-insensitive).
- On hit: write `.context/runs/<RID>/goal/REJECT.md` explaining the matched patterns (sorted list), store goal snapshot, schedule `synchronization` and EXIT.

- If the VERBATIM goal instructs writing to any locked path (per `.context/shared/locks/LOCKS.json`), do not write directly.
- Instead, write the requested payload to the appropriate PROPOSALS/ folder (e.g., `.context/shared/axioms/PROPOSALS/<sha12>.json`).
- Schedule `synchronization` to run admission.

- Derive `.context/runs/<RID>/goal/GOAL.header.json` (sorted keys; no timestamps) from the VERBATIM goal using deterministic heuristics:
  - "specificity": float in [0,1]; start at 0.5; +0.2 if code blocks detected; +0.15 if ≥3 explicit file paths; +0.1 if numbers/SLAs present; −0.2 if verbs {"improve","better","optimize","modernize"} dominate; clamp [0,1].
  - "scope": "local" if specificity≥0.7 and ≤2 paths; "module" if specificity≥0.5 or paths present; else "repo".
  - "relative_terms": sorted list found from {"good","better","best","optimal","fast","robust","scalable","performant","clean"}.
  - "acceptance_hints": true if goal contains "test", "assert", "expected", code fences with assertions, or uses "Definition of Done".
- Write to `.context/runs/<RID>/goal/GOAL.header.json` atomically (tmp+rename).
- Ensure `.context/goal/GOAL.md` exists and contains the **end-user prompt verbatim** (overwrite).
- Compute `RID` per RID Rules.
- Ensure `.context/runs/<RID>/goal/GOAL.md` exists and contains the same **verbatim** goal text (overwrite).

**STATE S1_PRECHECK (per this RID only)**
- If `.context/runs/<RID>/queue/next.json` exists and contains a recognized `task`, **IDLE** (do not overwrite).

**STATE S2_DEFAULT_SCHEDULE**
- Write a token to `.context/runs/<RID>/queue/next.json` with:
  - `run.id = RID`
  - `task = "planning"`
  - `subject = { "kind":"goal", "id": goal_hash, "hash": goal_hash }`
  - `requirements = {}`
  - `by = "system"`
  - `reason = "default_planning"`
- Append one line to `.context/runs/<RID>/STATUS.md`:
  `goal=<goal_hash> task=planning subject=goal id=<goal_hash> scheduled`

**STATE S3_EXIT**
- **EXIT**.

**Determinism**: no timestamps, no random IDs; always sort JSON keys; read files marked **VERBATIM** as exact text. Create directories using idempotent mkdir (no error if exists).


[PIN] DO NOT PERFORM WORK YOURSELF — schedule a single next agent token and stop


STATE S1_SCHEDULE
- Write `.context/runs/<RID>/next/TOKEN.txt` and STOP.


STATE S0b_TRUTH_ATTACH
- If `.context/stateless/knowledge/KNOWLEDGE_INDEX.json` missing: create default (corpus+medical) exactly.
- Write `.context/runs/<RID>/research/TRUTH_ATTACH.ptr` → `{"ref":"../../stateless/knowledge/KNOWLEDGE_INDEX.json"}`.


Provenance Gate
- Downstream artifacts MUST include `context_provenance` with `joint_context`, `packs`, and `rid`; reviews FAIL if missing.


AUTO-DEGRADE (Simple LLM)
- If an agent records 2 invalid writes OR 5 tool errors in the current run: schedule `synchronization` then `planning`; STOP.
