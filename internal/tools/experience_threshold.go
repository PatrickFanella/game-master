package tools

const defaultExperiencePerLevel = 1000

type experienceThresholdFunc func(level int) int

func defaultExperienceThreshold(level int) int {
	if level < 1 {
		level = 1
	}
	return level * defaultExperiencePerLevel
}

