package llm

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/openai"
)

/*
buildImageDataURL reads an image from disk and returns a data URL string that
can be used as the image_url field for OpenAI input_image content.
*/
func buildImageDataURL(imagePath string) (dataURL string, e *xerr.Error) {
	bytes, readErr := os.ReadFile(imagePath)
	if readErr != nil {
		e = xerr.NewError(readErr, "read image for LLM", imagePath)
		return "", e
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Safe default if extension is unknown.
		mimeType = "image/png"
	}

	encoded := base64.StdEncoding.EncodeToString(bytes)
	dataURL = fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

	return dataURL, nil
}

/*
GenerateReceiptAnalysisFromImage takes an image of a receipt, noisy OCR text,
and a list of regex-parsed price candidates, and produces a structured
ReceiptAnalysis using the OpenAI Responses API with vision.

Parameters:
  - imagePath: path to the original receipt image (photo).
  - ocrText: noisy OCR text extracted locally (often in Spanish).
  - priceCandidates: list of numeric price strings parsed via regex from
    numeric-only OCR (used as hints).
  - categories: optional category map (key -> description). If nil/empty, the
    default set of categories is used.

Behavior:
  - Sends the receipt image plus text (OCR + price list) to the model.
  - The model is instructed to:
    * read prices and items primarily from the image,
    * use OCR and priceCandidates as hints,
    * classify items into the provided categories,
    * compute totals and compare them.
*/
func GenerateReceiptAnalysisFromImage(
	imagePath string,
	ocrText string,
	priceCandidates []string,
	categories map[string]string,
) (receiptAnalysis ReceiptAnalysis, e *xerr.Error) {
	model := "gpt-5-mini"
	reasoningEffort := openai.EffortLow
	tools := []any{} // still disabling tools
	toolChoice := "auto"

	tl.Log(
		tl.Notice, palette.BlueBold, "%s with %s model %s, reasoning effort is %s",
		"Generating receipt analysis from image", "OpenAI", model, reasoningEffort,
	)

	imageDataURL, e := buildImageDataURL(imagePath)
	if e != nil {
		return receiptAnalysis, e
	}

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
	sort.Strings(categoryLines)
	categoryBlock := strings.Join(categoryLines, "\n")

	// Build the user text that accompanies the image: OCR + regex prices.
	var userTextBuilder strings.Builder
	userTextBuilder.WriteString("Below is noisy OCR text of a purchase receipt, followed by a list of regex-parsed price candidates.\n")
	userTextBuilder.WriteString("Use the attached receipt image as the primary source of truth; use the OCR text and price list only as hints.\n\n")

	userTextBuilder.WriteString("=== OCR TEXT START ===\n")
	userTextBuilder.WriteString(ocrText)
	userTextBuilder.WriteString("\n=== OCR TEXT END ===\n\n")

	userTextBuilder.WriteString("PRICE CANDIDATES:\n")
	if len(priceCandidates) == 0 {
		userTextBuilder.WriteString("- (none)\n")
	} else {
		for _, p := range priceCandidates {
			userTextBuilder.WriteString("- ")
			userTextBuilder.WriteString(p)
			userTextBuilder.WriteString("\n")
		}
	}

	userMessage := userTextBuilder.String()

	instructions := fmt.Sprintf(`
You are an assistant that parses noisy purchase receipts (often in Spanish)
using BOTH:
- a photo image of the receipt (attached as an input_image), and
- noisy OCR text plus a list of numeric price candidates (provided in the user message).

Your task:
- Carefully read the attached receipt image. Treat the IMAGE as the main ground truth.
- Use the OCR text and the "PRICE CANDIDATES" list only as hints for resolving ambiguous glyphs.
- Identify each purchased product line in the receipt.
- For each item, extract:
  - original_product_name: cleaned product name as it appears on the receipt (Spanish), without the price.
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

Additional hints:
- Receipts are in Colombian pesos (COP) and often use "." or "," as thousand separators but no cents.
- A trailing "A" after a price in the OCR often indicates a tax/IVA code and is not part of the numeric price.
- The list under "PRICE CANDIDATES" in the user message are likely price values from the receipt; prefer them when they are consistent with the image.
- Do NOT invent products that are not visually or textually implied by the receipt.
`, categoryBlock)

	developerMessage := `
Return only a single JSON object matching the provided schema.
Do not include any additional commentary or explanation outside the JSON.

Use the receipt IMAGE as the primary source of truth, especially for:
- which numbers belong in the PRECIO column,
- which lines correspond to actual items vs discounts or metadata.

Use the OCR text and price candidates only to help you decipher difficult characters,
but do not create items that do not appear visually on the receipt.
Perform a best-effort reconstruction of items and totals from the image + noisy text.
`

	// Same JSON schema properties as in GenerateReceiptAnalysis.
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
						"description": "Cleaned product name as it is in the OCR text/image, without the price.",
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

	// This wrapper needs to construct a Responses API request with:
	// - system + developer messages as input_text
	// - user: content = [ {type: "input_text", text: userMessage}, {type: "input_image", image_url: imageDataURL} ]
	receiptAnalysis, llmRunMetadata, e = openai.UseChatGPTResponsesAPIWithImage[ReceiptAnalysis](
		model,
		reasoningEffort,
		instructions,
		developerMessage,
		userMessage,
		imageDataURL,
		schemaProperties,
		4096,
		tools,
		toolChoice,
	)
	if e != nil {
		return receiptAnalysis, e
	}

	receiptAnalysis.LLMRunMetadata = llmRunMetadata

	tl.Log(
		tl.Notice1, palette.GreenBold, "%s with %s model %s, reasoning effort is %s",
		"Generated receipt analysis from image", "OpenAI", model, reasoningEffort,
	)
	tl.LogJSON(tl.Info, palette.Cyan, "OpenAI ReceiptAnalysis (image)", receiptAnalysis)

	return receiptAnalysis, nil
}
