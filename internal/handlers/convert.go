package handlers

import (
	"encoding/json"
	"time"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/engine"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/pkg/api"
)

func campaignToResponse(c statedb.Campaign) api.CampaignResponse {
	themes := c.Themes
	if themes == nil {
		themes = []string{}
	}
	rulesMode := c.RulesMode.String
	if rulesMode == "" {
		rulesMode = "narrative"
	}
	return api.CampaignResponse{
		ID:          dbutil.FromPgtype(c.ID).String(),
		Name:        c.Name,
		Description: c.Description.String,
		Genre:       c.Genre.String,
		Tone:        c.Tone.String,
		Themes:      themes,
		Status:      c.Status,
		RulesMode:   rulesMode,
		CreatedBy:   dbutil.FromPgtype(c.CreatedBy).String(),
		CreatedAt:   c.CreatedAt.Time,
		UpdatedAt:   c.UpdatedAt.Time,
	}
}

func playerCharacterToResponse(pc statedb.PlayerCharacter) api.CharacterResponse {
	stats := unmarshalJSONMap(pc.Stats)
	abilities := parseAbilities(pc.Abilities)

	var locID *string
	if pc.CurrentLocationID.Valid {
		s := dbutil.FromPgtype(pc.CurrentLocationID).String()
		locID = &s
	}

	return api.CharacterResponse{
		ID:                dbutil.FromPgtype(pc.ID).String(),
		CampaignID:        dbutil.FromPgtype(pc.CampaignID).String(),
		UserID:            dbutil.FromPgtype(pc.UserID).String(),
		Name:              pc.Name,
		Description:       pc.Description.String,
		Stats:             stats,
		HP:                int(pc.Hp),
		MaxHP:             int(pc.MaxHp),
		Experience:        int(pc.Experience),
		Level:             int(pc.Level),
		Status:            pc.Status,
		Abilities:         abilities,
		CurrentLocationID: locID,
	}
}

func locationToResponse(l statedb.Location, conns []statedb.GetConnectionsFromLocationRow) api.LocationResponse {
	connections := make([]api.LocationConnectionResponse, 0, len(conns))
	for _, c := range conns {
		connections = append(connections, api.LocationConnectionResponse{
			ToLocationID:  dbutil.FromPgtype(c.ConnectedLocationID).String(),
			Description:   c.Description.String,
			Bidirectional: c.Bidirectional,
			TravelTime:    c.TravelTime.String,
		})
	}
	return api.LocationResponse{
		ID:           dbutil.FromPgtype(l.ID).String(),
		CampaignID:   dbutil.FromPgtype(l.CampaignID).String(),
		Name:         l.Name,
		Description:  l.Description.String,
		Region:       l.Region.String,
		LocationType: l.LocationType.String,
		Properties:   unmarshalJSONMap(l.Properties),
		Connections:  connections,
	}
}

func npcToResponse(n statedb.Npc) api.NPCResponse {
	var factionID *string
	if n.FactionID.Valid {
		s := dbutil.FromPgtype(n.FactionID).String()
		factionID = &s
	}
	var hp *int
	if n.Hp.Valid {
		v := int(n.Hp.Int32)
		hp = &v
	}
	return api.NPCResponse{
		ID:          dbutil.FromPgtype(n.ID).String(),
		CampaignID:  dbutil.FromPgtype(n.CampaignID).String(),
		Name:        n.Name,
		Description: n.Description.String,
		Personality: n.Personality.String,
		Disposition: int(n.Disposition),
		FactionID:   factionID,
		Alive:       n.Alive,
		HP:          hp,
		Stats:       unmarshalJSONMap(n.Stats),
		Properties:  unmarshalJSONMap(n.Properties),
	}
}

func questToResponse(q statedb.Quest, objs []statedb.QuestObjective) api.QuestResponse {
	var parentID *string
	if q.ParentQuestID.Valid {
		s := dbutil.FromPgtype(q.ParentQuestID).String()
		parentID = &s
	}
	objectives := make([]api.QuestObjectiveResponse, 0, len(objs))
	for _, o := range objs {
		objectives = append(objectives, api.QuestObjectiveResponse{
			ID:          dbutil.FromPgtype(o.ID).String(),
			Description: o.Description,
			Completed:   o.Completed,
			OrderIndex:  int(o.OrderIndex),
		})
	}
	return api.QuestResponse{
		ID:            dbutil.FromPgtype(q.ID).String(),
		CampaignID:    dbutil.FromPgtype(q.CampaignID).String(),
		ParentQuestID: parentID,
		Title:         q.Title,
		Description:   q.Description.String,
		QuestType:     q.QuestType,
		Status:        q.Status,
		Objectives:    objectives,
	}
}

func itemToResponse(i statedb.Item) api.ItemResponse {
	var pcID *string
	if i.PlayerCharacterID.Valid {
		s := dbutil.FromPgtype(i.PlayerCharacterID).String()
		pcID = &s
	}
	return api.ItemResponse{
		ID:                dbutil.FromPgtype(i.ID).String(),
		CampaignID:        dbutil.FromPgtype(i.CampaignID).String(),
		PlayerCharacterID: pcID,
		Name:              i.Name,
		Description:       i.Description.String,
		ItemType:          i.ItemType,
		Rarity:            i.Rarity,
		Properties:        unmarshalJSONMap(i.Properties),
		Equipped:          i.Equipped,
		Quantity:          int(i.Quantity),
	}
}

func engineStateChangesToAPI(changes []engine.StateChange) []api.StateChange {
	out := make([]api.StateChange, 0, len(changes))
	for _, sc := range changes {
		details := make(map[string]any, 2)
		if sc.OldValue != nil {
			var old any
			if json.Unmarshal(sc.OldValue, &old) == nil {
				details["old_value"] = old
			}
		}
		if sc.NewValue != nil {
			var nv any
			if json.Unmarshal(sc.NewValue, &nv) == nil {
				details["new_value"] = nv
			}
		}
		out = append(out, api.StateChange{
			EntityType: sc.Entity,
			EntityID:   sc.EntityID.String(),
			ChangeType: sc.Field,
			Details:    details,
		})
	}
	return out
}

func engineTurnResultToAPI(tr *engine.TurnResult) api.TurnResponse {
	return api.TurnResponse{
		Narrative:    tr.Narrative,
		StateChanges: engineStateChangesToAPI(tr.StateChanges),
		CombatActive: tr.CombatActive,
	}
}

func sessionLogToEntry(sl statedb.SessionLog) api.SessionLogEntry {
	return api.SessionLogEntry{
		TurnNumber:  int(sl.TurnNumber),
		PlayerInput: sl.PlayerInput,
		InputType:   sl.InputType,
		LLMResponse: sl.LlmResponse,
		CreatedAt:   sl.CreatedAt.Time,
	}
}

// unmarshalJSONMap decodes raw JSON bytes into a map; returns empty map on
// nil/empty input or decode failure.
func unmarshalJSONMap(data []byte) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return map[string]any{}
	}
	return m
}

// parseAbilities decodes raw JSON bytes into a slice of CharacterAbility.
func parseAbilities(data []byte) []api.CharacterAbility {
	if len(data) == 0 {
		return []api.CharacterAbility{}
	}
	var abs []api.CharacterAbility
	if json.Unmarshal(data, &abs) != nil {
		return []api.CharacterAbility{}
	}
	return abs
}

func factToResponse(f statedb.WorldFact) api.FactResponse {
	var supersededBy *string
	if f.SupersededBy.Valid {
		s := dbutil.FromPgtype(f.SupersededBy).String()
		supersededBy = &s
	}
	return api.FactResponse{
		ID:           dbutil.FromPgtype(f.ID).String(),
		CampaignID:   dbutil.FromPgtype(f.CampaignID).String(),
		Fact:         f.Fact,
		Category:     f.Category,
		Source:       f.Source,
		SupersededBy: supersededBy,
		PlayerKnown:  f.PlayerKnown,
		CreatedAt:    f.CreatedAt.Time.Format(time.RFC3339),
	}
}

func relationshipToResponse(r statedb.EntityRelationship) api.RelationshipResponse {
	var strength *int
	if r.Strength.Valid {
		v := int(r.Strength.Int32)
		strength = &v
	}
	return api.RelationshipResponse{
		ID:               dbutil.FromPgtype(r.ID).String(),
		CampaignID:       dbutil.FromPgtype(r.CampaignID).String(),
		SourceEntityType: r.SourceEntityType,
		SourceEntityID:   dbutil.FromPgtype(r.SourceEntityID).String(),
		TargetEntityType: r.TargetEntityType,
		TargetEntityID:   dbutil.FromPgtype(r.TargetEntityID).String(),
		RelationshipType: r.RelationshipType,
		Description:      r.Description.String,
		Strength:         strength,
		PlayerAware:      r.PlayerAware,
		CreatedAt:        r.CreatedAt.Time.Format(time.RFC3339),
	}
}

func languageToResponse(l statedb.Language) api.LanguageResponse {
	return api.LanguageResponse{
		ID:          dbutil.FromPgtype(l.ID).String(),
		CampaignID:  dbutil.FromPgtype(l.CampaignID).String(),
		Name:        l.Name,
		Description: l.Description,
		PlayerKnown: l.PlayerKnown,
		CreatedAt:   l.CreatedAt.Time.Format(time.RFC3339),
	}
}

func cultureToResponse(c statedb.Culture) api.CultureResponse {
	var langID, beliefID *string
	if c.LanguageID.Valid {
		s := dbutil.FromPgtype(c.LanguageID).String()
		langID = &s
	}
	if c.BeliefSystemID.Valid {
		s := dbutil.FromPgtype(c.BeliefSystemID).String()
		beliefID = &s
	}
	return api.CultureResponse{
		ID:             dbutil.FromPgtype(c.ID).String(),
		CampaignID:     dbutil.FromPgtype(c.CampaignID).String(),
		Name:           c.Name,
		LanguageID:     langID,
		BeliefSystemID: beliefID,
		PlayerKnown:    c.PlayerKnown,
		CreatedAt:      c.CreatedAt.Time.Format(time.RFC3339),
	}
}

func beliefSystemToResponse(b statedb.BeliefSystem) api.BeliefSystemResponse {
	return api.BeliefSystemResponse{
		ID:          dbutil.FromPgtype(b.ID).String(),
		CampaignID:  dbutil.FromPgtype(b.CampaignID).String(),
		Name:        b.Name,
		PlayerKnown: b.PlayerKnown,
		CreatedAt:   b.CreatedAt.Time.Format(time.RFC3339),
	}
}

func economicSystemToResponse(e statedb.EconomicSystem) api.EconomicSystemResponse {
	return api.EconomicSystemResponse{
		ID:          dbutil.FromPgtype(e.ID).String(),
		CampaignID:  dbutil.FromPgtype(e.CampaignID).String(),
		Name:        e.Name,
		PlayerKnown: e.PlayerKnown,
		CreatedAt:   e.CreatedAt.Time.Format(time.RFC3339),
	}
}

func mapLocationToResponse(l statedb.Location) api.MapLocationResponse {
	return api.MapLocationResponse{
		ID:            dbutil.FromPgtype(l.ID).String(),
		CampaignID:    dbutil.FromPgtype(l.CampaignID).String(),
		Name:          l.Name,
		Description:   l.Description.String,
		Region:        l.Region.String,
		LocationType:  l.LocationType.String,
		PlayerVisited: l.PlayerVisited,
		PlayerKnown:   l.PlayerKnown,
	}
}
