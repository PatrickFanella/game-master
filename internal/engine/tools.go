package engine

import (
	"errors"

	"github.com/PatrickFanella/game-master/internal/game"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// toolMetas defines the category and rules-mode metadata for every tool,
// replacing the hardcoded category maps that previously lived in tool_filter.go.
var toolMetas = []tools.ToolMeta{
	// ── Base (always available) ──────────────────────────────────────
	{Name: "skill_check", Category: tools.CategoryBase},
	{Name: "roll_dice", Category: tools.CategoryBase},
	{Name: "move_player", Category: tools.CategoryBase},
	{Name: "describe_scene", Category: tools.CategoryBase},
	{Name: "present_choices", Category: tools.CategoryBase},
	{Name: "npc_dialogue", Category: tools.CategoryBase},
	{Name: "establish_fact", Category: tools.CategoryBase},
	{Name: "revise_fact", Category: tools.CategoryBase},
	{Name: "update_npc", Category: tools.CategoryBase},
	{Name: "add_item", Category: tools.CategoryBase},
	{Name: "remove_item", Category: tools.CategoryBase},
	{Name: "modify_item", Category: tools.CategoryBase},
	{Name: "create_item", Category: tools.CategoryBase},
	{Name: "generate_name", Category: tools.CategoryBase},
	{Name: "search_memory", Category: tools.CategoryBase},
	{Name: "advance_time", Category: tools.CategoryBase},
	{Name: "rest", Category: tools.CategoryBase},

	// ── Combat ──────────────────────────────────────────────────────
	// initiate_combat is CategoryExploration because it is available
	// during exploration (to start combat). The filter also explicitly
	// includes it during the combat phase for continuity.
	{Name: "initiate_combat", Category: tools.CategoryExploration},
	{Name: "combat_round", Category: tools.CategoryCombat},
	{Name: "apply_damage", Category: tools.CategoryCombat},
	{Name: "apply_condition", Category: tools.CategoryCombat},
	{Name: "resolve_combat", Category: tools.CategoryCombat},
	{Name: "add_ability", Category: tools.CategoryCombat},
	{Name: "remove_ability", Category: tools.CategoryCombat},
	{Name: "update_player_status", Category: tools.CategoryCombat},

	// ── Exploration ─────────────────────────────────────────────────
	{Name: "create_npc", Category: tools.CategoryExploration},
	{Name: "create_location", Category: tools.CategoryExploration},
	{Name: "create_city", Category: tools.CategoryExploration},
	{Name: "create_faction", Category: tools.CategoryExploration},
	{Name: "create_language", Category: tools.CategoryExploration},
	{Name: "create_culture", Category: tools.CategoryExploration},
	{Name: "create_belief_system", Category: tools.CategoryExploration},
	{Name: "create_economic_system", Category: tools.CategoryExploration},
	{Name: "create_lore", Category: tools.CategoryExploration},
	{Name: "establish_relationship", Category: tools.CategoryExploration},
	{Name: "reveal_location", Category: tools.CategoryExploration},

	// ── Quest ───────────────────────────────────────────────────────
	{Name: "create_quest", Category: tools.CategoryQuest},
	{Name: "create_subquest", Category: tools.CategoryQuest},
	{Name: "update_quest", Category: tools.CategoryQuest},
	{Name: "complete_objective", Category: tools.CategoryQuest},
	{Name: "branch_quest", Category: tools.CategoryQuest},
	{Name: "link_quest_entity", Category: tools.CategoryQuest},

	// ── Progression ─────────────────────────────────────────────────
	{Name: "add_experience", Category: tools.CategoryProgression},
	{Name: "level_up", Category: tools.CategoryProgression},
	{Name: "update_player_stats", Category: tools.CategoryProgression},

	// ── Crunch-only (rules mode restricted) ─────────────────────────
	{Name: "grant_feat", Category: tools.CategoryBase, RulesModes: []string{"crunch"}},
	{Name: "allocate_skill", Category: tools.CategoryBase, RulesModes: []string{"crunch"}},
}

// registerAllTools constructs every game service from the given querier and
// registers every tool with the supplied registry. Returns a joined error
// if any registration fails. The embedder may be nil; tools that support
// auto-embedding will skip it when nil.
func registerAllTools(registry *tools.Registry, queries statedb.Querier, embedder tools.Embedder, searcher tools.SearchMemorySearcher, timeDB ...tools.TimeStore) error {
	locSvc := game.NewLocationService(queries)
	invSvc := game.NewInventoryService(queries)
	npcSvc := game.NewNPCService(queries)
	worldSvc := game.NewWorldService(queries)
	combatSvc := game.NewCombatService(queries)
	progressionSvc := game.NewProgressionService(queries)
	statResolver := game.NewStatModifierResolver(queries)

	var errs []error
	if len(timeDB) > 0 && timeDB[0] != nil {
		errs = appendErr(errs, tools.RegisterMovePlayer(registry, locSvc, timeDB[0]))
	} else {
		errs = appendErr(errs, tools.RegisterMovePlayer(registry, locSvc))
	}
	errs = appendErr(errs, tools.RegisterAddItem(registry, invSvc))
	errs = appendErr(errs, tools.RegisterRemoveItem(registry, invSvc))
	errs = appendErr(errs, tools.RegisterModifyItem(registry, invSvc))
	errs = appendErr(errs, tools.RegisterCreateItem(registry, invSvc))
	errs = appendErr(errs, tools.RegisterRollDice(registry))
	errs = appendErr(errs, tools.RegisterUpdateNPC(registry, npcSvc))
	errs = appendErr(errs, tools.RegisterInitiateCombat(registry, combatSvc))
	errs = appendErr(errs, tools.RegisterCreateLanguage(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateBeliefSystem(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateEconomicSystem(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateCulture(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateCity(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateLocation(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateFaction(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateQuest(registry, worldSvc))
	errs = appendErr(errs, tools.RegisterEstablishRelationship(registry, worldSvc))
	errs = appendErr(errs, tools.RegisterCreateSubquest(registry, worldSvc))
	errs = appendErr(errs, tools.RegisterCompleteObjective(registry, worldSvc))
	errs = appendErr(errs, tools.RegisterUpdateQuest(registry, worldSvc))
	errs = appendErr(errs, tools.RegisterCreateNPC(registry, npcSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterGenerateName(registry, worldSvc))
	if len(timeDB) > 0 && timeDB[0] != nil {
		errs = appendErr(errs, tools.RegisterSkillCheck(registry, statResolver, nil, timeDB[0]))
	} else {
		errs = appendErr(errs, tools.RegisterSkillCheck(registry, statResolver, nil))
	}
	errs = appendErr(errs, tools.RegisterCombatRound(registry, nil))
	errs = appendErr(errs, tools.RegisterApplyDamage(registry))
	errs = appendErr(errs, tools.RegisterApplyCondition(registry))
	errs = appendErr(errs, tools.RegisterUpdatePlayerStats(registry, combatSvc))
	errs = appendErr(errs, tools.RegisterUpdatePlayerStatus(registry, combatSvc))
	errs = appendErr(errs, tools.RegisterAddExperience(registry, progressionSvc))
	errs = appendErr(errs, tools.RegisterLevelUp(registry, progressionSvc))
	errs = appendErr(errs, tools.RegisterAddAbility(registry, combatSvc))
	errs = appendErr(errs, tools.RegisterRemoveAbility(registry, combatSvc))
	errs = appendErr(errs, tools.RegisterResolveCombat(registry, combatSvc))
	errs = appendErr(errs, tools.RegisterEstablishFact(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterCreateLore(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterReviseFact(registry, worldSvc, worldSvc, embedder))
	errs = appendErr(errs, tools.RegisterDescribeScene(registry, locSvc))
	errs = appendErr(errs, tools.RegisterRevealLocation(registry, locSvc))
	errs = appendErr(errs, tools.RegisterNPCDialogue(registry, npcSvc))
	errs = appendErr(errs, tools.RegisterPresentChoices(registry))
	errs = appendErr(errs, tools.RegisterBranchQuest(registry, worldSvc))
	errs = appendErr(errs, tools.RegisterLinkQuestEntity(registry, worldSvc))
	if searcher != nil {
		errs = appendErr(errs, tools.RegisterSearchMemory(registry, searcher))
	}
	if len(timeDB) > 0 && timeDB[0] != nil {
		errs = appendErr(errs, tools.RegisterAdvanceTime(registry, timeDB[0]))
		errs = appendErr(errs, tools.RegisterRest(registry, timeDB[0]))
		// Register crunch-mode tools (visibility controlled by PhaseToolFilter).
		errs = appendErr(errs, tools.RegisterGrantFeat(registry, timeDB[0]))
		errs = appendErr(errs, tools.RegisterAllocateSkill(registry, timeDB[0]))
	}

	// Attach category metadata to every registered tool.
	for _, meta := range toolMetas {
		// SetMeta silently skips tools that were not registered above
		// (e.g. search_memory when searcher is nil, time-dependent tools
		// when timeDB is nil).
		if err := registry.SetMeta(meta); err != nil {
			// Not an error if the tool simply was not registered.
			continue
		}
	}

	return errors.Join(errs...)
}

// appendErr appends err to the slice only when non-nil.
func appendErr(errs []error, err error) []error {
	if err != nil {
		return append(errs, err)
	}
	return errs
}
