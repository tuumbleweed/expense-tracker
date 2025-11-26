// openairesp/websearch.go
package openai

import (
	"encoding/json"
)

// ---- Web Search tool ---------------------------------------------------------

// WebSearchFilters lets you allow-list or exclude domains.
// Note: allowed_domains is the one documented in OpenAI docs; exclude is optional here.
type WebSearchFilters struct {
	AllowedDomains  []string `json:"allowed_domains,omitempty"` // e.g., []{"ft.com","wsj.com"}
	ExcludedDomains []string `json:"excluded_domains,omitempty"`
}

// UserLocationApproximate follows the shape used in examples.
type UserLocationApproximate struct {
	Country  string `json:"country,omitempty"`  // ISO-3166 alpha-2, e.g. "US"
	City     string `json:"city,omitempty"`     // "Bogotá"
	Region   string `json:"region,omitempty"`   // "Cundinamarca"
	Timezone string `json:"timezone,omitempty"` // IANA tz, e.g. "America/Bogota"
}

// UserLocation wrapper (OpenAI examples show a "type":"approximate" envelope)
type UserLocation struct {
	Type        string                   `json:"type"` // "approximate"
	Approximate *UserLocationApproximate `json:"approximate,omitempty"`
}

// WebSearchTool is a typed representation of {"type":"web_search", ...}.
type WebSearchTool struct {
	Type string `json:"type"` // always "web_search"

	// Optional tuning knobs seen in examples:
	Filters           *WebSearchFilters `json:"filters,omitempty"`
	UserLocation      *UserLocation     `json:"user_location,omitempty"`
	SearchContextSize string            `json:"search_context_size,omitempty"` // "low" | "medium" | "high"
	// You can add more optional fields as OpenAI exposes them (e.g., recency_days) in the future.
}

// NewWebSearchTool gives you a sensible default ("medium" context).
func NewWebSearchTool() WebSearchTool {
	return WebSearchTool{
		Type:              "web_search",
		SearchContextSize: "medium",
	}
}

// MarshalJSON ensures we always emit {"type":"web_search", ...} and omit empties.
func (w WebSearchTool) MarshalJSON() ([]byte, error) {
	type alias WebSearchTool
	if w.Type == "" {
		w.Type = "web_search"
	}
	return json.Marshal(alias(w))
}

// ---- Helpers -----------------------------------------------------------------

// EnableWebSearchAllowedDomains returns a ready-to-append tool with an allow-list.
func EnableWebSearchAllowedDomains(domains []string) WebSearchTool {
	t := NewWebSearchTool()
	if len(domains) > 0 {
		t.Filters = &WebSearchFilters{AllowedDomains: domains}
	}
	return t
}
