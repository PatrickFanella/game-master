package domain

import (
	"regexp"
	"strings"
)

var inputWordPattern = regexp.MustCompile(`[\pL\pN]+`)

var metaActionKeywords = map[string]struct{}{
	"inventory": {},
	"character": {},
	"quest":     {},
	"save":      {},
	"quit":      {},
	"help":      {},
}

type InputClassifier struct{}

func Classify(input string) InputType {
	for _, token := range inputWordPattern.FindAllString(strings.ToLower(input), -1) {
		if _, ok := metaActionKeywords[token]; ok {
			return MetaAction
		}
	}

	return GameAction
}

func (InputClassifier) Classify(input string) InputType {
	return Classify(input)
}
