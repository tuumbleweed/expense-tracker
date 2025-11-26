package openai

import (
	"fmt"
	"strings"
	"time"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

/*
Extract AI run metadata to include in the report to the admin
*/
func ExtractLLMRunMetadata(resp responseObject, startTime time.Time) (meta LLMRunMetadata) {
	// Intent log — quote values that might be empty as requested.
	tl.Log(
		tl.Info, palette.Blue,
		"%s for response_id='%s' status='%s'",
		"Building LLMRunMetadata", resp.ID, resp.Status,
	)

	// Core identifiers & status
	meta.ResponseID = resp.ID
	meta.Status = resp.Status

	// Model snapshot parsing (YYYY-MM-DD at the end of model string)
	meta.Model, meta.ModelSnapshot = ParseModelSnapshot(resp.Model)

	// Reasoning effort normalization (map API strings to your Effort enum)
	if resp.Reasoning != nil {
		meta.ReasoningEffort = *resp.Reasoning.Effort
	}

	// Parameters
	meta.Temperature = resp.Temperature

	// Tokens (usage)
	if resp.Usage != nil {
		meta.TokensIn = resp.Usage.InputTokens
		meta.TokensOut = resp.Usage.OutputTokens
		meta.TokensTotal = resp.Usage.TotalTokens

		if resp.Usage.InputTokensDetails != nil {
			meta.TokensCached = resp.Usage.InputTokensDetails.CachedTokens
		}

		if resp.Usage.OutputTokensDetails != nil {
			meta.TokensReasoning = resp.Usage.OutputTokensDetails.ReasoningTokens
		}
	}

	// Timing: use startTime instead of CreatedAt (they truncate milliseconds) FinishedAt is "now".
	meta.StartedAt = startTime.UnixMilli()
	meta.FinishedAt = time.Now().UnixMilli()
	meta.Elapsed = meta.FinishedAt - meta.StartedAt

	meta.ResponseLogsUrl = fmt.Sprintf("https://platform.openai.com/logs/%s", meta.ResponseID)

	// Success
	tl.Log(
		tl.Info1, palette.Green,
		"%s for response_id='%s' status='%s'",
		"Built   LLMRunMetadata", meta.ResponseID, meta.Status,
	)
	return meta
}

/*
parseModelSnapshot splits a full model string into (base, snapshot).

Behavior:
  - If the string ends with a valid YYYY-MM-DD snapshot (e.g., "gpt-5-nano-2025-08-07"),
    it returns ("gpt-5-nano", "2025-08-07").
  - If no valid snapshot is found, it returns (model, "").

Examples:

	"gpt-5-nano-2025-08-07" -> ("gpt-5-nano", "2025-08-07")
	"gpt-5-nano"            -> ("gpt-5-nano", "")
	"gpt-5-nano-rc1"        -> ("gpt-5-nano-rc1", "")
*/
func ParseModelSnapshot(model string) (base string, snapshot string) {
	m := strings.TrimSpace(model)
	base, snapshot = m, ""

	// Fast path: ends with "-YYYY-MM-DD"
	if len(m) >= 11 {
		tail := m[len(m)-10:]
		// check date and preceding dash
		if _, err := time.Parse("2006-01-02", tail); err == nil && m[len(m)-11] == '-' {
			return m[:len(m)-11], tail
		}
	}

	// Fallback: last dash segment equals YYYY-MM-DD
	lastDash := strings.LastIndex(m, "-")
	if lastDash < 0 {
		return base, snapshot
	}
	candidate := m[lastDash+1:]
	if len(candidate) != len("2006-01-02") {
		return base, snapshot
	}
	if _, err := time.Parse("2006-01-02", candidate); err == nil {
		return m[:lastDash], candidate
	}

	return base, snapshot
}
