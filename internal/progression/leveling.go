package progression

const (
	DefaultExperiencePerLevel = 1000
	DefaultMaxLevel           = 20
)

// NextLevelThreshold returns the total experience required to advance from the
// current level to the next level using cumulative thresholds:
// level 1 -> 1000 total XP, level 2 -> 3000 total XP, level 3 -> 6000 total XP.
func NextLevelThreshold(level int) int {
	if level < 1 {
		level = 1
	}
	return DefaultExperiencePerLevel * level * (level + 1) / 2
}

// LevelFromExperience calculates character level from total experience using
// cumulative per-level thresholds and a configured level cap.
func LevelFromExperience(experience int, maxLevel int) int {
	if maxLevel < 1 {
		maxLevel = DefaultMaxLevel
	}
	threshold := 0
	for level := 1; level < maxLevel; level++ {
		threshold += level * DefaultExperiencePerLevel
		if experience < threshold {
			return level
		}
	}
	return maxLevel
}
