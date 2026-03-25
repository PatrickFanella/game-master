package game

import (
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

func userToDomain(u statedb.User) *domain.User {
	return &domain.User{
		ID:        dbutil.FromPgtype(u.ID),
		Name:      u.Name,
		CreatedAt: u.CreatedAt.Time,
		UpdatedAt: u.UpdatedAt.Time,
	}
}

func campaignToDomain(c statedb.Campaign) domain.Campaign {
	return domain.Campaign{
		ID:          dbutil.FromPgtype(c.ID),
		Name:        c.Name,
		Description: c.Description.String,
		Genre:       c.Genre.String,
		Tone:        c.Tone.String,
		Themes:      c.Themes,
		Status:      domain.CampaignStatus(c.Status),
		CreatedBy:   dbutil.FromPgtype(c.CreatedBy),
		CreatedAt:   c.CreatedAt.Time,
		UpdatedAt:   c.UpdatedAt.Time,
	}
}

func playerCharacterToDomain(pc statedb.PlayerCharacter) domain.PlayerCharacter {
	var locationID *uuid.UUID
	if pc.CurrentLocationID.Valid {
		id := dbutil.FromPgtype(pc.CurrentLocationID)
		locationID = &id
	}

	return domain.PlayerCharacter{
		ID:                dbutil.FromPgtype(pc.ID),
		CampaignID:        dbutil.FromPgtype(pc.CampaignID),
		UserID:            dbutil.FromPgtype(pc.UserID),
		Name:              pc.Name,
		Description:       pc.Description.String,
		Stats:             pc.Stats,
		HP:                int(pc.Hp),
		MaxHP:             int(pc.MaxHp),
		Experience:        int(pc.Experience),
		Level:             int(pc.Level),
		Status:            pc.Status,
		Abilities:         pc.Abilities,
		CurrentLocationID: locationID,
		CreatedAt:         pc.CreatedAt.Time,
		UpdatedAt:         pc.UpdatedAt.Time,
	}
}

func locationToDomain(l statedb.Location) domain.Location {
	return domain.Location{
		ID:           dbutil.FromPgtype(l.ID),
		CampaignID:   dbutil.FromPgtype(l.CampaignID),
		Name:         l.Name,
		Description:  l.Description.String,
		Region:       l.Region.String,
		LocationType: l.LocationType.String,
		Properties:   l.Properties,
		CreatedAt:    l.CreatedAt.Time,
		UpdatedAt:    l.UpdatedAt.Time,
	}
}

func locationConnectionToDomain(c statedb.GetConnectionsFromLocationRow) domain.LocationConnection {
	return domain.LocationConnection{
		ID:             dbutil.FromPgtype(c.ID),
		FromLocationID: dbutil.FromPgtype(c.FromLocationID),
		ToLocationID:   dbutil.FromPgtype(c.ToLocationID),
		Description:    c.Description.String,
		Bidirectional:  c.Bidirectional,
		TravelTime:     c.TravelTime.String,
		CampaignID:     dbutil.FromPgtype(c.CampaignID),
	}
}

func npcToDomain(n statedb.Npc) domain.NPC {
	var locationID *uuid.UUID
	if n.LocationID.Valid {
		id := dbutil.FromPgtype(n.LocationID)
		locationID = &id
	}

	var factionID *uuid.UUID
	if n.FactionID.Valid {
		id := dbutil.FromPgtype(n.FactionID)
		factionID = &id
	}

	var hp *int
	if n.Hp.Valid {
		v := int(n.Hp.Int32)
		hp = &v
	}

	return domain.NPC{
		ID:          dbutil.FromPgtype(n.ID),
		CampaignID:  dbutil.FromPgtype(n.CampaignID),
		Name:        n.Name,
		Description: n.Description.String,
		Personality: n.Personality.String,
		Disposition: int(n.Disposition),
		LocationID:  locationID,
		FactionID:   factionID,
		Alive:       n.Alive,
		HP:          hp,
		Stats:       n.Stats,
		Properties:  n.Properties,
		CreatedAt:   n.CreatedAt.Time,
		UpdatedAt:   n.UpdatedAt.Time,
	}
}

func questToDomain(q statedb.Quest) domain.Quest {
	var parentQuestID *uuid.UUID
	if q.ParentQuestID.Valid {
		id := dbutil.FromPgtype(q.ParentQuestID)
		parentQuestID = &id
	}

	return domain.Quest{
		ID:            dbutil.FromPgtype(q.ID),
		CampaignID:    dbutil.FromPgtype(q.CampaignID),
		ParentQuestID: parentQuestID,
		Title:         q.Title,
		Description:   q.Description.String,
		QuestType:     domain.QuestType(q.QuestType),
		Status:        domain.QuestStatus(q.Status),
		CreatedAt:     q.CreatedAt.Time,
		UpdatedAt:     q.UpdatedAt.Time,
	}
}

func questObjectiveToDomain(o statedb.QuestObjective) domain.QuestObjective {
	return domain.QuestObjective{
		ID:          dbutil.FromPgtype(o.ID),
		QuestID:     dbutil.FromPgtype(o.QuestID),
		Description: o.Description,
		Completed:   o.Completed,
		OrderIndex:  int(o.OrderIndex),
	}
}

func itemToDomain(i statedb.Item) domain.Item {
	var playerCharacterID *uuid.UUID
	if i.PlayerCharacterID.Valid {
		id := dbutil.FromPgtype(i.PlayerCharacterID)
		playerCharacterID = &id
	}

	return domain.Item{
		ID:                dbutil.FromPgtype(i.ID),
		CampaignID:        dbutil.FromPgtype(i.CampaignID),
		PlayerCharacterID: playerCharacterID,
		Name:              i.Name,
		Description:       i.Description.String,
		ItemType:          domain.ItemType(i.ItemType),
		Rarity:            i.Rarity,
		Properties:        i.Properties,
		Equipped:          i.Equipped,
		Quantity:          int(i.Quantity),
		CreatedAt:         i.CreatedAt.Time,
		UpdatedAt:         i.UpdatedAt.Time,
	}
}

func worldFactToDomain(f statedb.WorldFact) domain.WorldFact {
	var supersededBy *uuid.UUID
	if f.SupersededBy.Valid {
		id := dbutil.FromPgtype(f.SupersededBy)
		supersededBy = &id
	}

	return domain.WorldFact{
		ID:           dbutil.FromPgtype(f.ID),
		CampaignID:   dbutil.FromPgtype(f.CampaignID),
		Fact:         f.Fact,
		Category:     f.Category,
		Source:       f.Source,
		SupersededBy: supersededBy,
		CreatedAt:    f.CreatedAt.Time,
	}
}
