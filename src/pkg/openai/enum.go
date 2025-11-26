package openai

type TextVerbosity string

const (
	TextVerbosityLow    TextVerbosity = "low"
	textVerbosityMedium TextVerbosity = "medium" // (default behavior if omitted)
	textVerbosityHigh   TextVerbosity = "high"
)

type InputRole string

const (
	RoleDeveloper InputRole = "developer"
	RoleUser      InputRole = "user"
	RoleAssistant InputRole = "assistant"
	RoleTool      InputRole = "tool"
)

// Your organization must be verified to generate reasoning summaries.
type Summary string

// const (
// 	summaryAuto     summary = "auto"
// 	summaryDetailed summary = "detailed"
// )

type Effort string

const (
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
)
