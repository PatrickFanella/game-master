package tools

import "github.com/PatrickFanella/game-master/internal/progression"

type experienceThresholdFunc func(level int) int

func defaultExperienceThreshold(level int) int {
	return progression.NextLevelThreshold(level)
}
