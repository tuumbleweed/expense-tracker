/*
Parse receipt OCR output and classify each line item into categories using OpenAI.
*/
package llm

import (
	"fmt"
	"sort"
	"strings"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/openai"
)

/*
ReceiptItem holds information about a single product line parsed from a receipt.

Fields:
  - LineIndex: zero-based index of the line in the OCR text that primarily
    corresponds to this item. Use -1 if the line index is unclear.
  - RawLine: raw OCR text for this item (or the main line used).
  - OriginalProductName: cleaned product name as it is in receipt.
  - ProductNameEnglish: short English translation of the product name.
  - Quantity: quantity of the item (1.0 if not explicitly specified).
  - UnitPrice: unit price in COP, if you can infer it (0 if unknown).
  - LineTotal: total amount for this item in COP.
  - CategoryKey: one of the allowed category keys (or "other" if nothing fits).
*/
type ReceiptItem struct {
	LineIndex           int     `json:"line_index"`
	RawLine             string  `json:"raw_line"`
	OriginalProductName string  `json:"original_product_name"`
	ProductNameEnglish  string  `json:"product_name_english"`
	Quantity            float64 `json:"quantity"`
	UnitPrice           float64 `json:"unit_price"`
	LineTotal           float64 `json:"line_total"`
	CategoryKey         string  `json:"category_key"`
}

/*
ReceiptTotals holds the summary totals for a parsed receipt.

Fields:
  - ReceiptTotal: the total amount as written on the receipt (in COP).
  - ComputedItemsTotal: the sum of all item line totals (in COP).
*/
type ReceiptTotals struct {
	ReceiptTotal       float64 `json:"receipt_total"`
	ComputedItemsTotal float64 `json:"computed_items_total"`
	TotalCheckMessage  string  `json:"total_check_message"`
}

/*
ReceiptAnalysis is the full result of the AI-based receipt parsing.

Fields:
  - LLMRunMetadata: metadata returned by the OpenAI wrapper.
  - Items: list of parsed receipt items.
  - Categories: map of category keys to human-readable descriptions that were
    used for classification.
  - Totals: summary totals for the receipt (receipt total vs sum of items).
  - TotalCheckMessage: empty string if receipt total matches sum of items
    (within 1 COP); otherwise, a short English explanation of the difference.
*/
type ReceiptAnalysis struct {
	LLMRunMetadata *openai.LLMRunMetadata `json:"llm_run_metadata,omitempty"`
	Items          []ReceiptItem          `json:"items"`
	Totals         ReceiptTotals          `json:"totals"`
}

/*
buildDefaultReceiptCategories returns a map of reasonable default categories
for receipt items, keyed by a stable category key.

The descriptions are used both for prompt context and to expose the same map
to the caller via ReceiptAnalysis.Categories.
*/
func buildDefaultReceiptCategories() map[string]string {
	return map[string]string{
		"dairy":              "Milk, yogurt, cheese, butter and other dairy products.",
		"bakery":             "Bread, baked goods, pastries, tortillas and similar items.",
		"snacks_sweets":      "Snacks, cookies, candy, chocolate, desserts and similar.",
		"drinks_soft":        "Soft drinks, bottled water, juices and non-alcoholic beverages.",
		"other_food":         "Other food items not covered by more specific grocery categories.",
		"household_cleaning": "Cleaning products, detergents, paper goods and similar.",
		"personal_care":      "Toiletries, cosmetics, hygiene and personal care products.",
		"prepared_food":      "Prepared meals, take-out food or ready-to-eat items.",
		"medicine":           "Pharmacy products, medicines and health-related items.",
		"body_building":      "Protein, creatine etc.",
		"other":              "Anything that does not clearly fit any of the categories above.",
	}
}

/*
GenerateReceiptAnalysis takes OCR'd receipt text and an optional category map
and produces a structured ReceiptAnalysis using the OpenAI Responses API.

Parameters:
  - userMessage: raw OCR text from the receipt (possibly noisy, often in Spanish).
  - categories: optional category map (key -> description). If this map is
    nil or empty, a default set of categories is used.

Behavior:
  - The OCR text is sent to the model together with the list of allowed
    categories.
  - The model is instructed to:
  - Extract line items.
  - Normalize product names and amounts (in COP).
  - Assign each item to one of the categories; if no category fits,
    it must use "other".
  - Read the total amount from the receipt and compare it to the
    sum of all item line totals.
  - Set TotalCheckMessage to "" if the totals match within 1 COP,
    or to a short English explanation if they differ.
  - The returned ReceiptAnalysis includes:
  - Items
  - Totals
  - TotalCheckMessage
  - Categories (the effective category map used for the run)
  - LLMRunMetadata from the OpenAI wrapper.
*/
func GenerateReceiptAnalysis(userMessage string, categories map[string]string) (receiptAnalysis ReceiptAnalysis, e *xerr.Error) {
	model := "gpt-5-mini"
	reasoningEffort := openai.EffortLow
	tools := []any{} // disable the tools for now
	toolChoice := "auto"

	tl.Log(
		tl.Notice, palette.BlueBold, "%s with %s model %s, reasoning effort is %s",
		"Generating receipt analysis", "OpenAI", model, reasoningEffort,
	)

	// Ensure we have a category map; fall back to the default set if needed.
	effectiveCategories := categories
	if len(effectiveCategories) == 0 {
		effectiveCategories = buildDefaultReceiptCategories()
	}

	// Build a human-readable list of categories for the instructions.
	categoryLines := make([]string, 0, len(effectiveCategories))
	for key, description := range effectiveCategories {
		line := fmt.Sprintf("- %s: %s", key, description)
		categoryLines = append(categoryLines, line)
	}
	// Sort for determinism (helps debugging and testing).
	sort.Strings(categoryLines)
	categoryBlock := strings.Join(categoryLines, "\n")

	instructions := fmt.Sprintf(`
You are an assistant that parses noisy OCR text from purchase receipts (often in Spanish).

Your task:
- Read the OCR text from the user.
- Identify each purchased product line.
- For each item, extract:
  - original_product_name: cleaned product name exactly as in OCR text, without the price.
  - product_name_english: short English translation of the product name.
  - quantity: numeric quantity (use 1.0 if not explicitly given but implied).
  - unit_price: unit price in COP if you can infer it, otherwise 0.
  - line_total: total amount for that item in COP.
  - category_key: one of the allowed category keys listed below (or "other" if nothing fits).
- Compute and compare totals:
  - Determine receipt_total: the total amount charged according to the receipt (in COP).
  - Determine computed_items_total: sum of all item line_total values.
  - Compare them:
      * If they are equal within 1 COP, set total_check_message to "" (empty string).
      * Otherwise, set total_check_message to a short English explanation such as:
        "Sum of items is 10,470 COP but receipt total is 10,480 COP (difference: 10 COP)."

Allowed category keys and descriptions:
%s

Rules:
- category_key must be exactly one of the allowed category keys above.
- If no category clearly applies, use the key "other".
- Currency is Colombian pesos (COP).
- The OCR may be imperfect; fix obvious OCR mistakes but do not invent products that are not implied by the text.
`, categoryBlock)

	developerMessage := `
Return only a single JSON object matching the provided schema.
Do not include any additional commentary or explanation outside the JSON.
Perform a best-effort reconstruction of items and totals from the noisy OCR text.
`

	// JSON Schema fragment for Responses API (properties only).
	// This must match the ReceiptAnalysis struct layout.
	schemaProperties := map[string]any{
		"items": map[string]any{
			"type":        "array",
			"description": "List of line items parsed from the receipt.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"line_index": map[string]any{
						"type":        "integer",
						"description": "Zero-based index of the main OCR line for this item, or -1 if unknown.",
					},
					"raw_line": map[string]any{
						"type":        "string",
						"description": "Raw OCR text line(s) used to derive this item.",
					},
					"original_product_name": map[string]any{
						"type":        "string",
						"description": "Cleaned product name as it is in the OCR text, without the price.",
					},
					"product_name_english": map[string]any{
						"type":        "string",
						"description": "Short English translation of the product name.",
					},
					"quantity": map[string]any{
						"type":        "number",
						"description": "Quantity of the item (1.0 if not explicitly given).",
					},
					"unit_price": map[string]any{
						"type":        "number",
						"description": "Unit price in COP, or 0 if unknown.",
					},
					"line_total": map[string]any{
						"type":        "number",
						"description": "Total amount for this item in COP.",
					},
					"category_key": map[string]any{
						"type":        "string",
						"description": "One of the allowed category keys or 'other'.",
					},
				},
				"required": []string{
					"line_index",
					"raw_line",
					"original_product_name",
					"product_name_english",
					"quantity",
					"unit_price",
					"line_total",
					"category_key",
				},
				"additionalProperties": false,
			},
		},
		"totals": map[string]any{
			"type":        "object",
			"description": "Summary totals for the receipt.",
			"properties": map[string]any{
				"receipt_total": map[string]any{
					"type":        "number",
					"description": "Total amount as written on the receipt (in COP).",
				},
				"computed_items_total": map[string]any{
					"type":        "number",
					"description": "Sum of all item line_total values (in COP).",
				},
				"total_check_message": map[string]any{
					"type":        "string",
					"description": "Empty string if sums match within 1 COP; otherwise a short English explanation.",
				},
			},
			"required":             []string{"receipt_total", "computed_items_total", "total_check_message"},
			"additionalProperties": false,
		},
	}

	var llmRunMetadata *openai.LLMRunMetadata

	receiptAnalysis, llmRunMetadata, e = openai.UseChatGPTResponsesAPI[ReceiptAnalysis](
		model,
		reasoningEffort,
		instructions,
		developerMessage,
		userMessage,
		schemaProperties,
		4096,
		tools,
		toolChoice,
	)
	if e != nil {
		return receiptAnalysis, e
	}

	// Attach metadata and effective categories used for this run.
	receiptAnalysis.LLMRunMetadata = llmRunMetadata

	tl.Log(
		tl.Notice1, palette.GreenBold, "%s with %s model %s, reasoning effort is %s",
		"Generated receipt analysis", "OpenAI", model, reasoningEffort,
	)
	tl.LogJSON(tl.Info, palette.Cyan, "OpenAI ReceiptAnalysis", receiptAnalysis)

	return receiptAnalysis, nil
}
