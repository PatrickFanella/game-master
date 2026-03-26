package game

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

func TestUpdateNPCStoreLocationExistsInCampaign(t *testing.T) {
	locationID := uuid.New()
	campaignID := uuid.New()
	otherCampaignID := uuid.New()

	q := newMockQuerier()
	q.location = statedb.Location{
		ID:         dbutil.ToPgtype(locationID),
		CampaignID: dbutil.ToPgtype(campaignID),
	}
	store := NewUpdateNPCStore(q)

	ok, err := store.LocationExistsInCampaign(context.Background(), locationID, campaignID)
	if err != nil {
		t.Fatalf("LocationExistsInCampaign() error = %v", err)
	}
	if !ok {
		t.Fatal("expected location to exist in campaign")
	}

	ok, err = store.LocationExistsInCampaign(context.Background(), locationID, otherCampaignID)
	if err != nil {
		t.Fatalf("LocationExistsInCampaign() error = %v", err)
	}
	if ok {
		t.Fatal("expected location to be rejected for different campaign")
	}
}

func TestStringToPgText(t *testing.T) {
	if got := stringToPgText(""); got.Valid {
		t.Fatalf("stringToPgText(\"\") valid = %v, want false", got.Valid)
	}
	got := stringToPgText("hello")
	if !got.Valid || got.String != "hello" {
		t.Fatalf("stringToPgText(\"hello\") = %+v", got)
	}
}

func TestUpdateNPCStoreUpdateNPCPreservesNullTextForEmptyStrings(t *testing.T) {
	npcID := uuid.New()
	q := newMockQuerier()
	q.updateNPCResult = statedb.Npc{
		ID:         dbutil.ToPgtype(npcID),
		CampaignID: dbutil.ToPgtype(uuid.New()),
		Name:       "Null Keeper",
	}
	store := NewUpdateNPCStore(q)

	_, err := store.UpdateNPC(context.Background(), domain.NPC{
		ID:          npcID,
		CampaignID:  uuid.New(),
		Name:        "Null Keeper",
		Description: "",
		Personality: "",
		Disposition: 1,
		Alive:       true,
	})
	if err != nil {
		t.Fatalf("UpdateNPC() error = %v", err)
	}
	if q.lastUpdateNPCParams == nil {
		t.Fatal("expected UpdateNPC to be called")
	}
	if q.lastUpdateNPCParams.Description.Valid {
		t.Fatalf("description valid = %v, want false", q.lastUpdateNPCParams.Description.Valid)
	}
	if q.lastUpdateNPCParams.Personality.Valid {
		t.Fatalf("personality valid = %v, want false", q.lastUpdateNPCParams.Personality.Valid)
	}
}

func TestIntOrNullInt4(t *testing.T) {
	if got := intOrNullInt4(nil); got.Valid {
		t.Fatalf("intOrNullInt4(nil) valid = %v, want false", got.Valid)
	}
	v := 7
	got := intOrNullInt4(&v)
	if !got.Valid || got.Int32 != 7 {
		t.Fatalf("intOrNullInt4(&7) = %+v", got)
	}
}

func TestUpdateNPCStoreUpdateNPCPassesNullableFields(t *testing.T) {
	npcID := uuid.New()
	locationID := uuid.New()
	factionID := uuid.New()
	hp := 11

	q := newMockQuerier()
	q.updateNPCResult = statedb.Npc{
		ID:         dbutil.ToPgtype(npcID),
		CampaignID: dbutil.ToPgtype(uuid.New()),
		Name:       "Ranger",
	}
	store := NewUpdateNPCStore(q)

	_, err := store.UpdateNPC(context.Background(), domain.NPC{
		ID:          npcID,
		CampaignID:  uuid.New(),
		Name:        "Ranger",
		Description: "alert",
		Personality: "calm",
		Disposition: 9,
		LocationID:  &locationID,
		FactionID:   &factionID,
		Alive:       true,
		HP:          &hp,
	})
	if err != nil {
		t.Fatalf("UpdateNPC() error = %v", err)
	}
	if q.lastUpdateNPCParams == nil {
		t.Fatal("expected UpdateNPC to be called")
	}
	if !q.lastUpdateNPCParams.LocationID.Valid {
		t.Fatal("expected valid location_id")
	}
	if !q.lastUpdateNPCParams.FactionID.Valid {
		t.Fatal("expected valid faction_id")
	}
	if !q.lastUpdateNPCParams.Hp.Valid || q.lastUpdateNPCParams.Hp.Int32 != int32(hp) {
		t.Fatalf("expected hp=%d valid, got %+v", hp, q.lastUpdateNPCParams.Hp)
	}
}

func TestUpdateNPCStoreLocationExistsInCampaignNotFound(t *testing.T) {
	q := newMockQuerier()
	store := NewUpdateNPCStore(q)

	ok, err := store.LocationExistsInCampaign(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("LocationExistsInCampaign() error = %v", err)
	}
	if ok {
		t.Fatal("expected false when location does not exist")
	}
}

func TestStringToPgTextRoundTrip(t *testing.T) {
	value := "story"
	got := stringToPgText(value)
	if got != (pgtype.Text{String: value, Valid: true}) {
		t.Fatalf("unexpected pgtype.Text: %+v", got)
	}
}
