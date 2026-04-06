package engine

import (
	"errors"

	"github.com/PatrickFanella/game-master/internal/game"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// registerAllTools constructs every game service from the given querier and
// registers every tool with the supplied registry. Returns a joined error
// if any registration fails. The embedder may be nil; tools that support
// auto-embedding will skip it when nil.
func registerAllTools(registry *tools.Registry, queries statedb.Querier, embedder tools.Embedder, searcher tools.SearchMemorySearcher) error {
	locSvc := game.NewLocationService(queries)
	invSvc := game.NewInventoryService(queries)
	npcSvc := game.NewNPCService(queries)
	worldSvc := game.NewWorldService(queries)
	combatSvc := game.NewCombatService(queries)
	progressionSvc := game.NewProgressionService(queries)
	statResolver := game.NewStatModifierResolver(queries)

	var errs []error
	errs = appendErr(errs, tools.RegisterMovePlayer(registry, locSvc))
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
	errs = appendErr(errs, tools.RegisterSkillCheck(registry, statResolver, nil))
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
	return errors.Join(errs...)
}

// appendErr appends err to the slice only when non-nil.
func appendErr(errs []error, err error) []error {
	if err != nil {
		return append(errs, err)
	}
	return errs
}
