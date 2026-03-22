---
title: Phase 1 Execution Paths — Foundation & Infrastructure
type: tracking
created: 2026-03-21
tags: [tracking, phase-1, execution, foundation]
---

# Phase 1: Foundation & Infrastructure

> 54 issues across 6 tracks. **10 ready now**, 44 blocked by dependencies.
> Updated: 2026-03-21

## Summary

| Track | Name             | Total  | Ready  | Blocked | Epic | Models              |
| ----- | ---------------- | :----: | :----: | :-----: | ---- | ------------------- |
| A     | Project Scaffold |   10   |   10   |    0    | #2   | gpt-5.4 mini / Codex |
| B     | Database Schema  |   14   |   0    |   14    | #3   | gpt-5.3-codex       |
| C     | Database Queries |   12   |   0    |   12    | #3   | gpt-5.3-codex       |
| D     | Database Tests   |   1    |   0    |    1    | #3   | Claude Sonnet 4.6   |
| E     | LLM Provider     |   7    |   0    |    7    | #4   | Mixed               |
| F     | TUI Shell        |   10   |   0    |   10    | #5   | Mixed               |
|       | **Total**        | **54** | **10** | **44**  |      |                     |

**Critical path:** Track A → Tracks B, E, F (parallel) → Track C → Track D

**Phase exit criteria:** All 54 issues closed. Postgres+pgvector running, sqlc generating code, LLM provider making calls, TUI shell booting with view switching, CI green.

---

## Track A: Project Scaffold

> Foundation for everything. All issues are independent and parallelizable.
> Depends on: Nothing

| #   | Issue                                                          | Title                                         | Size | Blocker | Status | Model         | Notes                                   |
| --- | -------------------------------------------------------------- | --------------------------------------------- | :--: | ------- | ------ | ------------- | --------------------------------------- |
| 1   | [#19](https://github.com/PatrickFanella/game-master/issues/19) | Initialize Go module and directory structure  |  S   | None    | READY  | gpt-5.4 mini  | Do first — everything imports from this |
| 2   | [#20](https://github.com/PatrickFanella/game-master/issues/20) | Create Docker Compose: Postgres with pgvector |  S   | None    | READY  | gpt-5.4 mini  |                                         |
| 3   | [#21](https://github.com/PatrickFanella/game-master/issues/21) | Add Ollama service to Docker Compose          |  XS  | None    | READY  | gpt-5.4 mini  |                                         |
| 4   | [#22](https://github.com/PatrickFanella/game-master/issues/22) | Create Taskfile with core tasks               |  S   | None    | READY  | gpt-5.4 mini  |                                         |
| 5   | [#23](https://github.com/PatrickFanella/game-master/issues/23) | Configure goose for database migrations       |  S   | None    | READY  | gpt-5.4 mini  |                                         |
| 6   | [#24](https://github.com/PatrickFanella/game-master/issues/24) | Configure sqlc for code generation            |  S   | None    | READY  | gpt-5.4 mini  |                                         |
| 7   | [#25](https://github.com/PatrickFanella/game-master/issues/25) | Set up koanf configuration loading            |  M   | None    | READY  | gpt-5.3-codex |                                         |
| 8   | [#26](https://github.com/PatrickFanella/game-master/issues/26) | Create GitHub Actions CI workflow             |  M   | None    | READY  | gpt-5.4 mini  |                                         |
| 9   | [#27](https://github.com/PatrickFanella/game-master/issues/27) | Create .gitignore and .env.example            |  XS  | None    | READY  | Claude Haiku 4.5 |                                      |
| 10  | [#28](https://github.com/PatrickFanella/game-master/issues/28) | Create README with setup instructions         |  S   | None    | READY  | Claude Sonnet 4.6 |                                    |

```mermaid
graph LR
    19[#19 Go module + dirs] --> B[Track B: DB Schema]
    19 --> E[Track E: LLM Provider]
    19 --> F[Track F: TUI Shell]
    20[#20 Docker Compose PG] --> B
    21[#21 Docker Compose Ollama] --> E
    22[#22 Taskfile] --> B
    23[#23 Goose config] --> B
    24[#24 sqlc config] --> C[Track C: DB Queries]
    25[#25 koanf config] --> E
    25 --> F
    26[#26 CI workflow]
    27[#27 .gitignore]
    28[#28 README]

    style 19 fill:#22c55e
    style 20 fill:#22c55e
    style 21 fill:#22c55e
    style 22 fill:#22c55e
    style 23 fill:#22c55e
    style 24 fill:#22c55e
    style 25 fill:#22c55e
    style 26 fill:#22c55e
    style 27 fill:#22c55e
    style 28 fill:#22c55e
```

**Parallelizable:** All 10 issues can run simultaneously. Prioritize #19 (everything depends on it).

---

## Track B: Database Schema (Migrations)

> All table migrations. Must run after Track A scaffold is in place.
> Depends on: Track A (#19 go module, #20 Docker Compose, #23 goose)

| #   | Issue                                                          | Title                                        | Size | Blocker  | Status  | Model         | Notes                     |
| --- | -------------------------------------------------------------- | -------------------------------------------- | :--: | -------- | ------- | ------------- | ------------------------- |
| 1   | [#29](https://github.com/PatrickFanella/game-master/issues/29) | Migration: enable pgvector extension         |  XS  | Track A  | BLOCKED | gpt-5.3-codex | Must be first migration   |
| 2   | [#30](https://github.com/PatrickFanella/game-master/issues/30) | Migration: create users table                |  XS  | #29      | BLOCKED | gpt-5.3-codex |                           |
| 3   | [#31](https://github.com/PatrickFanella/game-master/issues/31) | Migration: create campaigns table            |  XS  | #30      | BLOCKED | gpt-5.3-codex | FK → users                |
| 4   | [#33](https://github.com/PatrickFanella/game-master/issues/33) | Migration: create locations table            |  XS  | #31      | BLOCKED | gpt-5.3-codex | FK → campaigns            |
| 5   | [#36](https://github.com/PatrickFanella/game-master/issues/36) | Migration: create factions table             |  S   | #31      | BLOCKED | gpt-5.3-codex | FK → campaigns            |
| 6   | [#34](https://github.com/PatrickFanella/game-master/issues/34) | Migration: create location_connections table |  XS  | #33      | BLOCKED | gpt-5.3-codex | FK → locations            |
| 7   | [#35](https://github.com/PatrickFanella/game-master/issues/35) | Migration: create npcs table                 |  S   | #33, #36 | BLOCKED | gpt-5.3-codex | FK → locations, factions  |
| 8   | [#32](https://github.com/PatrickFanella/game-master/issues/32) | Migration: create player_characters table    |  S   | #31, #33 | BLOCKED | gpt-5.3-codex | FK → campaigns, locations |
| 9   | [#37](https://github.com/PatrickFanella/game-master/issues/37) | Migration: create items table                |  S   | #32      | BLOCKED | gpt-5.3-codex | FK → player_characters    |
| 10  | [#38](https://github.com/PatrickFanella/game-master/issues/38) | Migration: create quests table               |  S   | #31      | BLOCKED | gpt-5.3-codex | FK → campaigns            |
| 11  | [#39](https://github.com/PatrickFanella/game-master/issues/39) | Migration: create world_facts table          |  XS  | #31      | BLOCKED | gpt-5.3-codex | FK → campaigns            |
| 12  | [#40](https://github.com/PatrickFanella/game-master/issues/40) | Migration: create session_logs table         |  S   | #31, #33 | BLOCKED | gpt-5.3-codex | FK → campaigns, locations |
| 13  | [#41](https://github.com/PatrickFanella/game-master/issues/41) | Migration: create memories table (vector)    |  S   | #29, #31 | BLOCKED | gpt-5.3-codex | Needs pgvector extension  |
| 14  | [#42](https://github.com/PatrickFanella/game-master/issues/42) | Migration: create entity_relationships table |  XS  | #31      | BLOCKED | gpt-5.3-codex | FK → campaigns            |

```mermaid
graph TD
    A[Track A complete] --> 29[#29 pgvector ext]
    29 --> 30[#30 users]
    30 --> 31[#31 campaigns]
    31 --> 33[#33 locations]
    31 --> 36[#36 factions]
    31 --> 38[#38 quests]
    31 --> 39[#39 world_facts]
    31 --> 42[#42 entity_rels]
    33 --> 34[#34 location_conns]
    33 --> 35[#35 npcs]
    36 --> 35
    31 --> 32[#32 player_chars]
    33 --> 32
    32 --> 37[#37 items]
    31 --> 40[#40 session_logs]
    33 --> 40
    29 --> 41[#41 memories]
    31 --> 41

    style 29 fill:#ef4444
    style 30 fill:#ef4444
    style 31 fill:#ef4444
    style 32 fill:#ef4444
    style 33 fill:#ef4444
    style 34 fill:#ef4444
    style 35 fill:#ef4444
    style 36 fill:#ef4444
    style 37 fill:#ef4444
    style 38 fill:#ef4444
    style 39 fill:#ef4444
    style 40 fill:#ef4444
    style 41 fill:#ef4444
    style 42 fill:#ef4444
```

**Parallelizable after #31:** Issues #33, #36, #38, #39, #42 can all start once campaigns table exists. Second wave: #32, #34, #35, #37, #40, #41.

---

## Track C: Database Queries (sqlc)

> All sqlc query files. Can be written once their target tables exist.
> Depends on: Track B (migrations), Track A (#24 sqlc config)

| #   | Issue                                                          | Title                                         | Size | Blocker  | Status  | Model           | Notes                           |
| --- | -------------------------------------------------------------- | --------------------------------------------- | :--: | -------- | ------- | --------------- | ------------------------------- |
| 1   | [#43](https://github.com/PatrickFanella/game-master/issues/43) | sqlc queries: users CRUD                      |  S   | #30      | BLOCKED | gpt-5.3-codex   |                                 |
| 2   | [#44](https://github.com/PatrickFanella/game-master/issues/44) | sqlc queries: campaigns CRUD                  |  S   | #31      | BLOCKED | gpt-5.3-codex   |                                 |
| 3   | [#45](https://github.com/PatrickFanella/game-master/issues/45) | sqlc queries: player_characters CRUD          |  S   | #32      | BLOCKED | gpt-5.3-codex   |                                 |
| 4   | [#46](https://github.com/PatrickFanella/game-master/issues/46) | sqlc queries: locations and connections CRUD  |  S   | #33, #34 | BLOCKED | gpt-5.3-codex   |                                 |
| 5   | [#47](https://github.com/PatrickFanella/game-master/issues/47) | sqlc queries: npcs CRUD                       |  S   | #35      | BLOCKED | gpt-5.3-codex   |                                 |
| 6   | [#48](https://github.com/PatrickFanella/game-master/issues/48) | sqlc queries: factions and relationships CRUD |  S   | #36      | BLOCKED | gpt-5.3-codex   |                                 |
| 7   | [#49](https://github.com/PatrickFanella/game-master/issues/49) | sqlc queries: items CRUD                      |  S   | #37      | BLOCKED | gpt-5.3-codex   |                                 |
| 8   | [#50](https://github.com/PatrickFanella/game-master/issues/50) | sqlc queries: quests and objectives CRUD      |  S   | #38      | BLOCKED | gpt-5.3-codex   |                                 |
| 9   | [#51](https://github.com/PatrickFanella/game-master/issues/51) | sqlc queries: world_facts CRUD                |  S   | #39      | BLOCKED | gpt-5.3-codex   |                                 |
| 10  | [#52](https://github.com/PatrickFanella/game-master/issues/52) | sqlc queries: session_logs CRUD               |  S   | #40      | BLOCKED | gpt-5.3-codex   |                                 |
| 11  | [#53](https://github.com/PatrickFanella/game-master/issues/53) | sqlc queries: memories with vector search     |  M   | #41      | BLOCKED | Claude Sonnet 4.6 | Critical for semantic retrieval |
| 12  | [#54](https://github.com/PatrickFanella/game-master/issues/54) | sqlc queries: entity_relationships CRUD       |  S   | #42      | BLOCKED | gpt-5.3-codex   |                                 |

**Parallelizable:** All 12 query files can be written simultaneously once their corresponding migrations exist. Each is independent.

---

## Track D: Database Integration Tests

> Validates the entire data layer works end-to-end.
> Depends on: Track B (all migrations), Track C (all queries)

| #   | Issue                                                          | Title                                          | Size | Blocker    | Status  | Model           | Notes             |
| --- | -------------------------------------------------------------- | ---------------------------------------------- | :--: | ---------- | ------- | --------------- | ----------------- |
| 1   | [#55](https://github.com/PatrickFanella/game-master/issues/55) | Integration tests: migrations and sqlc queries |  L   | Track B, C | BLOCKED | Claude Sonnet 4.6 | testcontainers-go |

---

## Track E: LLM Provider

> Provider-agnostic LLM interface + Ollama implementation. Runs parallel to Tracks B-D.
> Depends on: Track A (#19 go module, #21 Ollama Docker Compose, #25 koanf)

| #   | Issue                                                          | Title                                          | Size | Blocker | Status  | Model             | Notes                  |
| --- | -------------------------------------------------------------- | ---------------------------------------------- | :--: | ------- | ------- | ----------------- | ---------------------- |
| 1   | [#56](https://github.com/PatrickFanella/game-master/issues/56) | Define LLMProvider interface and core types    |  S   | Track A | BLOCKED | Claude Opus 4.6   | Do first in this track |
| 2   | [#57](https://github.com/PatrickFanella/game-master/issues/57) | Implement Ollama HTTP client                   |  M   | #56     | BLOCKED | gpt-5.3-codex     |                        |
| 3   | [#58](https://github.com/PatrickFanella/game-master/issues/58) | Implement Ollama tool definition serialization |  S   | #56     | BLOCKED | gpt-5.3-codex     |                        |
| 4   | [#59](https://github.com/PatrickFanella/game-master/issues/59) | Implement Ollama tool call parsing             |  S   | #57     | BLOCKED | gpt-5.3-codex     |                        |
| 5   | [#60](https://github.com/PatrickFanella/game-master/issues/60) | Implement Ollama streaming support             |  M   | #57     | BLOCKED | Claude Sonnet 4.6 |                        |
| 6   | [#61](https://github.com/PatrickFanella/game-master/issues/61) | Implement LLM provider error handling          |  S   | #56     | BLOCKED | gpt-5.3-codex     |                        |
| 7   | [#62](https://github.com/PatrickFanella/game-master/issues/62) | Unit tests: LLM provider with fixtures         |  M   | #57-#61 | BLOCKED | gpt-5.3-codex     |                        |

```mermaid
graph TD
    A[Track A complete] --> 56[#56 LLMProvider interface]
    56 --> 57[#57 Ollama HTTP client]
    56 --> 58[#58 Tool serialization]
    56 --> 61[#61 Error handling]
    57 --> 59[#59 Tool call parsing]
    57 --> 60[#60 Streaming]
    57 --> 62[#62 Unit tests]
    58 --> 62
    59 --> 62
    60 --> 62
    61 --> 62

    style 56 fill:#ef4444
    style 57 fill:#ef4444
    style 58 fill:#ef4444
    style 59 fill:#ef4444
    style 60 fill:#ef4444
    style 61 fill:#ef4444
    style 62 fill:#ef4444
```

**Parallelizable after #56:** #57, #58, #61 can start simultaneously. After #57: #59 and #60 in parallel.

---

## Track F: TUI Shell

> Bubble Tea application shell. Runs parallel to Tracks B-E.
> Depends on: Track A (#19 go module, #25 koanf)

| #   | Issue                                                          | Title                                       | Size | Blocker  | Status  | Model             | Notes                                   |
| --- | -------------------------------------------------------------- | ------------------------------------------- | :--: | -------- | ------- | ----------------- | --------------------------------------- |
| 1   | [#70](https://github.com/PatrickFanella/game-master/issues/70) | Implement Lip Gloss styling and layout      |  M   | Track A  | BLOCKED | Claude Sonnet 4.6 | Do first — other views depend on styles |
| 2   | [#63](https://github.com/PatrickFanella/game-master/issues/63) | Create cmd/tui entry point and app init     |  S   | Track A  | BLOCKED | gpt-5.3-codex     |                                         |
| 3   | [#64](https://github.com/PatrickFanella/game-master/issues/64) | Create root Bubble Tea model                |  M   | #63, #70 | BLOCKED | Claude Sonnet 4.6 |                                         |
| 4   | [#65](https://github.com/PatrickFanella/game-master/issues/65) | Create narrative view: scrolling viewport   |  M   | #64, #70 | BLOCKED | Claude Sonnet 4.6 |                                         |
| 5   | [#66](https://github.com/PatrickFanella/game-master/issues/66) | Create narrative view: text input component |  S   | #64, #70 | BLOCKED | gpt-5.3-codex     |                                         |
| 6   | [#67](https://github.com/PatrickFanella/game-master/issues/67) | Create character sheet placeholder view     |  XS  | #64, #70 | BLOCKED | gpt-5.4 mini      |                                         |
| 7   | [#68](https://github.com/PatrickFanella/game-master/issues/68) | Create inventory placeholder view           |  XS  | #64, #70 | BLOCKED | gpt-5.4 mini      |                                         |
| 8   | [#69](https://github.com/PatrickFanella/game-master/issues/69) | Create quest log placeholder view           |  XS  | #64, #70 | BLOCKED | gpt-5.4 mini      |                                         |
| 9   | [#71](https://github.com/PatrickFanella/game-master/issues/71) | Implement tab switching with status bar     |  S   | #64-#69  | BLOCKED | gpt-5.3-codex     | Needs all views                         |
| 10  | [#72](https://github.com/PatrickFanella/game-master/issues/72) | teatest tests for TUI shell                 |  M   | #71      | BLOCKED | Claude Sonnet 4.6 |                                         |

```mermaid
graph TD
    A[Track A complete] --> 70[#70 Lip Gloss styling]
    A --> 63[#63 cmd/tui entry]
    63 --> 64[#64 Root model]
    70 --> 64
    64 --> 65[#65 Narrative viewport]
    64 --> 66[#66 Text input]
    64 --> 67[#67 Character placeholder]
    64 --> 68[#68 Inventory placeholder]
    64 --> 69[#69 Quest placeholder]
    70 --> 65
    70 --> 66
    70 --> 67
    70 --> 68
    70 --> 69
    65 --> 71[#71 Tab switching]
    66 --> 71
    67 --> 71
    68 --> 71
    69 --> 71
    71 --> 72[#72 teatest tests]

    style 70 fill:#ef4444
    style 63 fill:#ef4444
    style 64 fill:#ef4444
    style 65 fill:#ef4444
    style 66 fill:#ef4444
    style 67 fill:#ef4444
    style 68 fill:#ef4444
    style 69 fill:#ef4444
    style 71 fill:#ef4444
    style 72 fill:#ef4444
```

**Parallelizable:** #63 and #70 in parallel first. Then #65-#69 all in parallel after #64+#70.

---

## Phase 1 Execution Order

```
Week 1:  Track A (all 10 issues in parallel)
         ├── Commit: scaffold, Docker Compose, Taskfile, CI, config
         └── Gate: `docker compose up` works, `task test` runs, CI green

Week 2:  Track B + Track E + Track F (all in parallel)
         ├── Track B: migrations #29→#30→#31→(fan out)
         ├── Track E: #56→(#57, #58, #61 parallel)→(#59, #60)→#62
         └── Track F: (#63, #70 parallel)→#64→(#65-#69 parallel)→#71→#72

Week 3:  Track C (all 12 query files in parallel) + Track D
         ├── Track C: all sqlc queries (once migrations are done)
         └── Track D: integration tests (#55)
         └── Gate: all queries generate, integration tests pass, CI green
```

**Phase 1 → Phase 2 handoff:** Tracks B+C+D (database), Track E (LLM), and Track F (TUI) must all be complete before Phase 2 can begin.
