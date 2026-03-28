---
title: Phase 4 Execution Paths — Depth & Polish
type: tracking
created: 2026-03-28
tags: [tracking, phase-4, execution, gameplay]
---

# Phase 4: Depth & Polish

> 62 remaining issues across 8 tracks. Reorganized from the original Phase 3 plan after Phase 2 completion and early Phase 3 combat/worldbuilding work.
> Updated: 2026-03-28

## What's Done

Phases 1-2 complete. From the original Phase 3, the following tracks are **finished**:

- **Combat** (Track G): NarrativeCombatResolver + 5 combat tools (#139-#147 closed)
- **Expanded World** (Track H): 6 worldbuilding tools + generate_name (#149-#157 closed)
- **Claude Provider** (Phase 2 Track C): Full implementation (#168-#175 closed)
- **Context Assembly** (Track B partial): Tier 1+2 + unified assembler (#98, #99, #101 closed)

## Phase Entry Criteria

Turn pipeline working end-to-end: player types input in TUI, LLM responds with narrative + tool calls, state changes persist, response streams to viewport. (**Met.**)

## Phase Exit Criteria

Full gameplay depth: LLM can create/modify all game entities during play. Semantic memory provides relevant context. Campaign creation is guided. TUI views show real data. REST API serves game state.

---

## Sprint 1: Gameplay Tools (LLM needs these to run the game)

> The LLM currently can narrate, move the player, manage NPCs, handle combat, and do worldbuilding — but it can't modify player stats, create world entities on the fly, or manage quests. These are the tools that make gameplay rich.

### Track A: Player Character Tools

> Tools for the LLM to modify the player during gameplay.
> Depends on: Nothing (all sqlc queries exist)

| #   | Issue                                                            | Title                                                   | Size | Blocker | Status |
| --- | ---------------------------------------------------------------- | ------------------------------------------------------- | :--: | ------- | ------ |
| 1   | [#113](https://github.com/PatrickFanella/game-master/issues/113) | Implement update_player_stats tool                      |  S   | None    | READY  |
| 2   | [#114](https://github.com/PatrickFanella/game-master/issues/114) | Implement add_experience and level_up tools             |  M   | None    | READY  |
| 3   | [#115](https://github.com/PatrickFanella/game-master/issues/115) | Implement add_ability and remove_ability tools          |  S   | None    | READY  |
| 4   | [#116](https://github.com/PatrickFanella/game-master/issues/116) | Implement update_player_status tool                     |  S   | None    | READY  |
| 5   | [#117](https://github.com/PatrickFanella/game-master/issues/117) | Implement enhanced item tools: modify_item, create_item |  S   | None    | READY  |

**All 5 can run in parallel.** Follow existing tool handler pattern (see `internal/tools/doc.go`). Store interfaces use domain types; adapters in `game/`.

### Track B: World Generation Tools

> Tools for the LLM to create world entities during gameplay.
> Depends on: Nothing (all sqlc queries exist)

| #   | Issue                                                            | Title                                          | Size | Blocker | Status |
| --- | ---------------------------------------------------------------- | ---------------------------------------------- | :--: | ------- | ------ |
| 1   | [#122](https://github.com/PatrickFanella/game-master/issues/122) | Implement create_npc tool                      |  M   | None    | READY  |
| 2   | [#123](https://github.com/PatrickFanella/game-master/issues/123) | Implement create_location tool                 |  M   | None    | READY  |
| 3   | [#124](https://github.com/PatrickFanella/game-master/issues/124) | Implement create_faction tool                  |  M   | None    | READY  |
| 4   | [#125](https://github.com/PatrickFanella/game-master/issues/125) | Implement create_lore tool                     |  S   | None    | READY  |
| 5   | [#126](https://github.com/PatrickFanella/game-master/issues/126) | Implement establish_fact and revise_fact tools |  M   | None    | READY  |
| 6   | [#127](https://github.com/PatrickFanella/game-master/issues/127) | Implement establish_relationship tool          |  S   | None    | READY  |

**All 6 can run in parallel.** These are the "just-in-time" creation tools — the LLM generates NPCs, locations, and lore as the player encounters them.

### Track C: Quest System Core

> Tools for the LLM to drive story structure through quests.
> Depends on: Nothing (all sqlc queries exist)

| #   | Issue                                                            | Title                              | Size | Blocker | Status  |
| --- | ---------------------------------------------------------------- | ---------------------------------- | :--: | ------- | ------- |
| 1   | [#131](https://github.com/PatrickFanella/game-master/issues/131) | Implement create_quest tool        |  M   | None    | READY   |
| 2   | [#132](https://github.com/PatrickFanella/game-master/issues/132) | Implement create_subquest tool     |  S   | #131    | BLOCKED |
| 3   | [#133](https://github.com/PatrickFanella/game-master/issues/133) | Implement update_quest tool        |  S   | None    | READY   |
| 4   | [#135](https://github.com/PatrickFanella/game-master/issues/135) | Implement quest objective tracking |  M   | #131    | BLOCKED |

**#131 and #133 can start immediately. After #131: #132 and #135 in parallel.**

```mermaid
graph TD
    S1[Sprint 1 Start] --> A[Track A: Player Tools<br>5 parallel]
    S1 --> B[Track B: World Gen<br>6 parallel]
    S1 --> C131[#131 create_quest]
    S1 --> C133[#133 update_quest]
    C131 --> C132[#132 create_subquest]
    C131 --> C135[#135 objective tracking]
```

**Sprint 1 total: 15 issues, all tool implementations. Maximum parallelism.**

---

## Sprint 2: Memory & Context (LLM gets smarter over time)

> The LLM currently has a 10-turn sliding window and full game state. This sprint adds semantic memory (recall relevant past events) and summarization (compress old turns).

### Track D: Embedding Pipeline

> Semantic memory infrastructure. Unlocks Tier 3 context and auto-embedding.
> Depends on: Nothing (pgvector schema already exists)

| #   | Issue                                                          | Title                                             | Size | Blocker | Status  |
| --- | -------------------------------------------------------------- | ------------------------------------------------- | :--: | ------- | ------- |
| 1   | [#91](https://github.com/PatrickFanella/game-master/issues/91) | Define Embedder interface and types               |  S   | None    | READY   |
| 2   | [#92](https://github.com/PatrickFanella/game-master/issues/92) | Implement Ollama embedding provider               |  M   | #91     | BLOCKED |
| 3   | [#93](https://github.com/PatrickFanella/game-master/issues/93) | Implement turn event embedding and storage        |  M   | #92     | BLOCKED |
| 4   | [#94](https://github.com/PatrickFanella/game-master/issues/94) | Implement semantic search by similarity           |  M   | #91     | BLOCKED |
| 5   | [#95](https://github.com/PatrickFanella/game-master/issues/95) | Implement metadata-filtered semantic search       |  M   | #94     | BLOCKED |
| 6   | [#96](https://github.com/PatrickFanella/game-master/issues/96) | Implement async embedding pipeline                |  L   | #92     | BLOCKED |
| 7   | [#97](https://github.com/PatrickFanella/game-master/issues/97) | Integration tests: pgvector storage and retrieval |  L   | #94     | BLOCKED |

**#91 first, then #92 and #94 in parallel. After #92: #93, #96. After #94: #95, #97.**

### Track E: Context Management

> Better LLM context: semantic retrieval, summarization, token budgets.
> Depends on: Track D (#91, #94 for semantic search)

| #   | Issue                                                            | Title                                            | Size | Blocker     | Status  |
| --- | ---------------------------------------------------------------- | ------------------------------------------------ | :--: | ----------- | ------- |
| 1   | [#100](https://github.com/PatrickFanella/game-master/issues/100) | Implement Tier 3 context: semantic retrieval     |  M   | Track D #94 | BLOCKED |
| 2   | [#102](https://github.com/PatrickFanella/game-master/issues/102) | Implement turn summarization via LLM             |  M   | None        | READY   |
| 3   | [#103](https://github.com/PatrickFanella/game-master/issues/103) | Implement rich metadata extraction for summaries |  M   | #102        | BLOCKED |
| 4   | [#104](https://github.com/PatrickFanella/game-master/issues/104) | Implement sliding window aging and summarization |  M   | #102        | BLOCKED |
| 5   | [#105](https://github.com/PatrickFanella/game-master/issues/105) | Implement token budget awareness                 |  M   | None        | READY   |

**#102 and #105 can start immediately.** #100 waits for Track D's semantic search. After #102: #103 and #104 in parallel.

### Track F: Advanced Quest + World

> Quest branching, entity linking, validation, auto-embedding.
> Depends on: Sprint 1 (Track C quest tools), Track D (for auto-embedding)

| #   | Issue                                                            | Title                                           | Size | Blocker          | Status  |
| --- | ---------------------------------------------------------------- | ----------------------------------------------- | :--: | ---------------- | ------- |
| 1   | [#134](https://github.com/PatrickFanella/game-master/issues/134) | Implement branch_quest tool                     |  M   | Sprint 1 #131    | BLOCKED |
| 2   | [#136](https://github.com/PatrickFanella/game-master/issues/136) | Implement quest-entity linking                  |  M   | Sprint 1 #131    | BLOCKED |
| 3   | [#128](https://github.com/PatrickFanella/game-master/issues/128) | Implement post-hoc entity validation            |  M   | Sprint 1 Track B | BLOCKED |
| 4   | [#129](https://github.com/PatrickFanella/game-master/issues/129) | Implement auto-embedding for generated entities |  M   | Track D #92      | BLOCKED |

```mermaid
graph TD
    S1[Sprint 1 Complete] --> D91[#91 Embedder interface]
    D91 --> D92[#92 Ollama embedder]
    D91 --> D94[#94 Semantic search]
    D92 --> D93[#93 Turn embedding]
    D92 --> D96[#96 Async pipeline]
    D94 --> D95[#95 Filtered search]
    D94 --> D97[#97 Integration tests]
    D94 --> E100[#100 Tier 3 context]

    S1 --> E102[#102 Summarization]
    S1 --> E105[#105 Token budget]
    E102 --> E103[#103 Metadata extraction]
    E102 --> E104[#104 Window aging]

    S1 --> F134[#134 branch_quest]
    S1 --> F136[#136 quest-entity linking]
    S1 --> F128[#128 post-hoc validation]
    D92 --> F129[#129 auto-embedding]
```

**Sprint 2 total: 18 issues. Track D is the critical path.**

---

## Sprint 3: User Experience (polish the TUI, guided creation)

> Replace placeholder views with real data. Add structured campaign creation.

### Track G: TUI Views

> Fill in the placeholder character sheet, inventory, and quest views.
> Depends on: Sprint 1 (player + quest tools provide the data)

| #   | Issue                                                            | Title                                            | Size | Blocker          | Status  |
| --- | ---------------------------------------------------------------- | ------------------------------------------------ | :--: | ---------------- | ------- |
| 1   | [#118](https://github.com/PatrickFanella/game-master/issues/118) | Build character sheet TUI view                   |  M   | Sprint 1 Track A | BLOCKED |
| 2   | [#119](https://github.com/PatrickFanella/game-master/issues/119) | Build inventory TUI view                         |  M   | Sprint 1 Track A | BLOCKED |
| 3   | [#137](https://github.com/PatrickFanella/game-master/issues/137) | Build quest log TUI view                         |  M   | Sprint 1 Track C | BLOCKED |
| 4   | [#120](https://github.com/PatrickFanella/game-master/issues/120) | Implement real-time view updates on state change |  L   | #118, #119, #137 | BLOCKED |

**#118, #119, #137 in parallel. Then #120 once all views exist.**

### Track H: Campaign Creation

> Guided campaign + character creation using LLM interviews and Huh forms.
> Depends on: Sprint 1 Track B (world gen tools for skeleton generation)

| #   | Issue                                                            | Title                                              | Size | Blocker          | Status  |
| --- | ---------------------------------------------------------------- | -------------------------------------------------- | :--: | ---------------- | ------- |
| 1   | [#106](https://github.com/PatrickFanella/game-master/issues/106) | Implement campaign creation LLM interview flow     |  L   | None             | READY   |
| 2   | [#107](https://github.com/PatrickFanella/game-master/issues/107) | Implement Huh forms for structured campaign inputs |  M   | #106             | BLOCKED |
| 3   | [#108](https://github.com/PatrickFanella/game-master/issues/108) | Implement character creation interview             |  L   | #106             | BLOCKED |
| 4   | [#109](https://github.com/PatrickFanella/game-master/issues/109) | Implement world skeleton generation                |  L   | Sprint 1 Track B | BLOCKED |
| 5   | [#110](https://github.com/PatrickFanella/game-master/issues/110) | Implement starting scene generation                |  M   | #109             | BLOCKED |
| 6   | [#111](https://github.com/PatrickFanella/game-master/issues/111) | Implement campaign selection screen                |  M   | None             | READY   |
| 7   | [#112](https://github.com/PatrickFanella/game-master/issues/112) | Implement campaign resume and state restoration    |  M   | #111             | BLOCKED |

**#106 and #111 start immediately.** #109 waits for world gen tools.

### Track I: Tests

> Unit and contract tests for all Sprint 1-2 work.

| #   | Issue                                                            | Title                                            | Size | Blocker          | Status  |
| --- | ---------------------------------------------------------------- | ------------------------------------------------ | :--: | ---------------- | ------- |
| 1   | [#121](https://github.com/PatrickFanella/game-master/issues/121) | Unit tests: player character and inventory tools |  L   | Sprint 1 Track A | BLOCKED |
| 2   | [#130](https://github.com/PatrickFanella/game-master/issues/130) | Unit tests: world generation tools               |  L   | Sprint 1 Track B | BLOCKED |
| 3   | [#138](https://github.com/PatrickFanella/game-master/issues/138) | Unit tests: quest system tools                   |  L   | Sprint 1 Track C | BLOCKED |
| 4   | [#148](https://github.com/PatrickFanella/game-master/issues/148) | Interface contract tests for CombatResolver      |  L   | None             | READY   |

**Sprint 3 total: 15 issues. TUI views and campaign creation are the big UX wins.**

---

## Sprint 4: API Layer (multiplayer foundation)

> REST API + WebSocket streaming for future web/mobile clients.
> Depends on: Sprint 1 (needs working game engine for endpoints)

### Track J: REST API

| #   | Issue                                                            | Title                                        | Size | Blocker   | Status  |
| --- | ---------------------------------------------------------------- | -------------------------------------------- | :--: | --------- | ------- |
| 1   | [#158](https://github.com/PatrickFanella/game-master/issues/158) | Create cmd/server entry point and chi router |  M   | None      | READY   |
| 2   | [#166](https://github.com/PatrickFanella/game-master/issues/166) | Implement auth middleware interface (no-op)  |  S   | None      | READY   |
| 3   | [#160](https://github.com/PatrickFanella/game-master/issues/160) | Implement campaign REST endpoints            |  M   | #158      | BLOCKED |
| 4   | [#161](https://github.com/PatrickFanella/game-master/issues/161) | Implement character REST endpoints           |  S   | #158      | BLOCKED |
| 5   | [#162](https://github.com/PatrickFanella/game-master/issues/162) | Implement location and NPC REST endpoints    |  M   | #158      | BLOCKED |
| 6   | [#163](https://github.com/PatrickFanella/game-master/issues/163) | Implement quest REST endpoints               |  S   | #158      | BLOCKED |
| 7   | [#164](https://github.com/PatrickFanella/game-master/issues/164) | Implement POST /action endpoint              |  M   | #158      | BLOCKED |
| 8   | [#165](https://github.com/PatrickFanella/game-master/issues/165) | Implement WebSocket streaming endpoint       |  L   | #158      | BLOCKED |
| 9   | [#167](https://github.com/PatrickFanella/game-master/issues/167) | HTTP integration tests                       |  L   | #160-#165 | BLOCKED |

**#158 and #166 start in parallel. After #158: #160-#165 all in parallel. #167 last.**

**Sprint 4 total: 9 issues.**

---

## Remaining Epics

| Epic | Title                                          | Status       | Remaining Issues |
| ---- | ---------------------------------------------- | ------------ | ---------------- |
| #7   | Embedding pipeline + semantic memory           | Sprint 2     | #91-#97          |
| #8   | Context window management + turn summarization | Sprint 2     | #100, #102-#105  |
| #9   | Collaborative campaign creation                | Sprint 3     | #106-#112        |
| #10  | Player character management + inventory        | Sprint 1+3   | #113-#121        |
| #11  | World generation tools                         | Sprint 1+2   | #122-#130        |
| #12  | Quest system                                   | Sprint 1+2+3 | #131-#138        |
| #15  | REST API + WebSocket streaming                 | Sprint 4     | #158-#167        |

---

## Full Dependency Graph

```mermaid
graph TD
    subgraph "Sprint 1: Gameplay Tools"
        A[Track A: Player Tools<br>#113-#117<br>5 parallel]
        B[Track B: World Gen<br>#122-#127<br>6 parallel]
        C[Track C: Quest Core<br>#131-#133, #135]
    end

    subgraph "Sprint 2: Memory & Context"
        D[Track D: Embedding<br>#91-#97]
        E[Track E: Context<br>#100, #102-#105]
        F[Track F: Advanced Quest+World<br>#128-#129, #134, #136]
    end

    subgraph "Sprint 3: UX Polish"
        G[Track G: TUI Views<br>#118-#120, #137]
        H[Track H: Campaign Creation<br>#106-#112]
        I[Track I: Tests<br>#121, #130, #138, #148]
    end

    subgraph "Sprint 4: API"
        J[Track J: REST API<br>#158-#167]
    end

    A --> G
    B --> H
    C --> F
    C --> G
    D --> E
    D --> F
    A --> I
    B --> I
    C --> I

    style A fill:#22c55e
    style B fill:#22c55e
    style C fill:#22c55e
    style D fill:#ef4444
    style E fill:#ef4444
    style F fill:#ef4444
    style G fill:#ef4444
    style H fill:#ef4444
    style I fill:#ef4444
    style J fill:#ef4444
```

**Green = ready now. Red = blocked by earlier sprints.**

---

## Summary

| Sprint    | Focus            | Issues | Ready  | Blocked |
| --------- | ---------------- | :----: | :----: | :-----: |
| 1         | Gameplay Tools   |   15   | **15** |    0    |
| 2         | Memory & Context |   18   |   3    |   15    |
| 3         | UX Polish        |   15   |   3    |   12    |
| 4         | API Layer        |   9    |   2    |    7    |
| **Total** |                  | **57** | **23** | **34**  |

**Critical path:** Sprint 1 (all tools) → Sprint 2 Track D (embedding) → Sprint 2 Track E (#100 semantic retrieval) → Sprint 3 (TUI views + campaign creation)

**Maximum parallelism in Sprint 1:** All 15 tool issues can be built simultaneously — they share the same pattern and have zero interdependencies (except #132 and #135 depending on #131).
