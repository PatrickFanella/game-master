package game

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

func uuidFromPgtype(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: u != uuid.Nil}
}

func userToDomain(u statedb.User) *domain.User {
	return &domain.User{
		ID:        uuidFromPgtype(u.ID),
		Name:      u.Name,
		CreatedAt: u.CreatedAt.Time,
		UpdatedAt: u.UpdatedAt.Time,
	}
}

func campaignToDomain(c statedb.Campaign) domain.Campaign {
	return domain.Campaign{
		ID:          uuidFromPgtype(c.ID),
		Name:        c.Name,
		Description: c.Description.String,
		Genre:       c.Genre.String,
		Tone:        c.Tone.String,
		Themes:      c.Themes,
		Status:      domain.CampaignStatus(c.Status),
		CreatedBy:   uuidFromPgtype(c.CreatedBy),
		CreatedAt:   c.CreatedAt.Time,
		UpdatedAt:   c.UpdatedAt.Time,
	}
}

func playerCharacterToDomain(pc statedb.PlayerCharacter) domain.PlayerCharacter {
	var locationID *uuid.UUID
	if pc.CurrentLocationID.Valid {
		id := uuidFromPgtype(pc.CurrentLocationID)
		locationID = &id
	}

	return domain.PlayerCharacter{
		ID:                uuidFromPgtype(pc.ID),
		CampaignID:        uuidFromPgtype(pc.CampaignID),
		UserID:            uuidFromPgtype(pc.UserID),
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
		ID:           uuidFromPgtype(l.ID),
		CampaignID:   uuidFromPgtype(l.CampaignID),
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
		ID:             uuidFromPgtype(c.ID),
		FromLocationID: uuidFromPgtype(c.FromLocationID),
		ToLocationID:   uuidFromPgtype(c.ToLocationID),
		Description:    c.Description.String,
		Bidirectional:  c.Bidirectional,
		TravelTime:     c.TravelTime.String,
		CampaignID:     uuidFromPgtype(c.CampaignID),
	}
}

func npcToDomain(n statedb.Npc) domain.NPC {
	var locationID *uuid.UUID
	if n.LocationID.Valid {
		id := uuidFromPgtype(n.LocationID)
		locationID = &id
	}

	var factionID *uuid.UUID
	if n.FactionID.Valid {
		id := uuidFromPgtype(n.FactionID)
		factionID = &id
	}

	var hp *int
	if n.Hp.Valid {
		v := int(n.Hp.Int32)
		hp = &v
	}

	return domain.NPC{
		ID:          uuidFromPgtype(n.ID),
		CampaignID:  uuidFromPgtype(n.CampaignID),
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
		id := uuidFromPgtype(q.ParentQuestID)
		parentQuestID = &id
	}

	return domain.Quest{
		ID:            uuidFromPgtype(q.ID),
		CampaignID:    uuidFromPgtype(q.CampaignID),
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
		ID:          uuidFromPgtype(o.ID),
		QuestID:     uuidFromPgtype(o.QuestID),
		Description: o.Description,
		Completed:   o.Completed,
		OrderIndex:  int(o.OrderIndex),
	}
}

func itemToDomain(i statedb.Item) domain.Item {
	var playerCharacterID *uuid.UUID
	if i.PlayerCharacterID.Valid {
		id := uuidFromPgtype(i.PlayerCharacterID)
		playerCharacterID = &id
	}

	return domain.Item{
		ID:                uuidFromPgtype(i.ID),
		CampaignID:        uuidFromPgtype(i.CampaignID),
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
		id := uuidFromPgtype(f.SupersededBy)
		supersededBy = &id
	}

	return domain.WorldFact{
		ID:           uuidFromPgtype(f.ID),
		CampaignID:   uuidFromPgtype(f.CampaignID),
		Fact:         f.Fact,
		Category:     f.Category,
		Source:       f.Source,
		SupersededBy: supersededBy,
		CreatedAt:    f.CreatedAt.Time,
	}
}
