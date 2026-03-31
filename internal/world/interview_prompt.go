package world

import _ "embed"

// interviewPrompt is the system prompt that instructs the LLM how to conduct
// the campaign-creation interview.
//
//go:embed interview_prompt.txt
var interviewPrompt string
