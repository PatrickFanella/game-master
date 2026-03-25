package combat

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"
)

type initiativeRoller interface {
	RollD20() int
	Intn(n int) int
}

type defaultInitiativeRoller struct {
	rng *rand.Rand
}

const initiativeTieBreakRange = 1 << 30

func (r *defaultInitiativeRoller) RollD20() int {
	return r.rng.Intn(20) + 1
}

func (r *defaultInitiativeRoller) Intn(n int) int {
	return r.rng.Intn(n)
}

func newDefaultInitiativeRoller() initiativeRoller {
	seed := time.Now().UnixNano()
	var cryptSeed int64
	if err := binary.Read(cryptorand.Reader, binary.LittleEndian, &cryptSeed); err == nil {
		seed = cryptSeed
	}

	return &defaultInitiativeRoller{
		rng: rand.New(rand.NewSource(seed)),
	}
}

type combatantStat struct {
	Dexterity int `json:"dexterity"`
}

func dexterityStat(combatant Combatant) int {
	if len(combatant.Stats) == 0 {
		return 10
	}

	var stat combatantStat
	if err := json.Unmarshal(combatant.Stats, &stat); err != nil {
		return 10
	}

	if stat.Dexterity == 0 {
		return 10
	}

	return stat.Dexterity
}

func dexterityModifier(combatant Combatant) int {
	dex := dexterityStat(combatant)
	// Standard d20-style ability modifier:
	//   dex >= 10: (dex-10)/2
	//   dex < 10:  -((11-dex)/2)
	if dex >= 10 {
		return (dex - 10) / 2
	}

	return -((11 - dex) / 2)
}

// RollInitiative assigns initiative to all combatants using d20 + dexterity
// modifier and persists initiative order to combat state.
func RollInitiative(combatState *CombatState) error {
	return rollInitiativeWithRoller(combatState, newDefaultInitiativeRoller())
}

func rollInitiativeWithRoller(combatState *CombatState, roller initiativeRoller) error {
	if combatState == nil {
		return fmt.Errorf("combat state is required")
	}
	if roller == nil {
		return fmt.Errorf("initiative roller is required")
	}
	if len(combatState.Combatants) == 0 {
		return fmt.Errorf("combat state must have at least one combatant")
	}

	for i := range combatState.Combatants {
		roll := roller.RollD20()
		combatState.Combatants[i].Initiative = roll + dexterityModifier(combatState.Combatants[i])
	}

	sortCombatantsByInitiative(combatState.Combatants, roller)
	combatState.InitiativeOrder = make([]uuid.UUID, len(combatState.Combatants))
	for i := range combatState.Combatants {
		combatState.InitiativeOrder[i] = combatState.Combatants[i].EntityID
	}

	return nil
}

// StartNextRound increments round counter and re-rolls initiative when
// configured to do so.
func StartNextRound(combatState *CombatState) error {
	return startNextRoundWithRoller(combatState, newDefaultInitiativeRoller())
}

func startNextRoundWithRoller(combatState *CombatState, roller initiativeRoller) error {
	if combatState == nil {
		return fmt.Errorf("combat state is required")
	}
	if roller == nil {
		return fmt.Errorf("initiative roller is required")
	}

	combatState.RoundNumber++
	combatState.SurpriseRoundActive = combatState.RoundNumber == 1 && hasSurprisedCombatant(combatState.Combatants)

	if combatState.RoundNumber == 1 || combatState.InitiativeRerollEachRound {
		return rollInitiativeWithRoller(combatState, roller)
	}

	return nil
}

// CombatantsForCurrentRound returns combatants in initiative order, skipping
// surprised combatants during the active surprise round.
func CombatantsForCurrentRound(combatState *CombatState) []Combatant {
	if combatState == nil {
		return nil
	}

	combatants := make([]Combatant, 0, len(combatState.Combatants))
	for i := range combatState.Combatants {
		if combatState.SurpriseRoundActive && combatState.Combatants[i].Surprised {
			continue
		}
		combatants = append(combatants, combatState.Combatants[i])
	}

	return combatants
}

func hasSurprisedCombatant(combatants []Combatant) bool {
	for i := range combatants {
		if combatants[i].Surprised {
			return true
		}
	}

	return false
}

func sortCombatantsByInitiative(combatants []Combatant, roller initiativeRoller) {
	randomTieBreak := make(map[uuid.UUID]int, len(combatants))
	for i := range combatants {
		// Use a large range to reduce accidental collisions while staying
		// comfortably within platform int bounds.
		randomTieBreak[combatants[i].EntityID] = roller.Intn(initiativeTieBreakRange)
	}

	sort.SliceStable(combatants, func(i, j int) bool {
		if combatants[i].Initiative != combatants[j].Initiative {
			return combatants[i].Initiative > combatants[j].Initiative
		}

		dexI := dexterityStat(combatants[i])
		dexJ := dexterityStat(combatants[j])
		if dexI != dexJ {
			return dexI > dexJ
		}

		randI := randomTieBreak[combatants[i].EntityID]
		randJ := randomTieBreak[combatants[j].EntityID]
		if randI != randJ {
			return randI > randJ
		}

		return combatants[i].EntityID.String() > combatants[j].EntityID.String()
	})
}
