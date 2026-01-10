package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

/*
receiptRun represents a single parsed JSON output file produced by the pipeline.

It is intentionally permissive: unknown fields are ignored, and some optional
date fields are supported if present in the JSON.
*/
type receiptRun struct {
	LLMRunMetadata llmRunMetadata `json:"llm_run_metadata"`
	Items          []receiptItem  `json:"items"`
	Totals         receiptTotals  `json:"totals"`

	ReceiptDate     string `json:"receipt_date"`
	ReceiptDateTime string `json:"receipt_datetime"`
}

/*
llmRunMetadata captures the run metadata from the JSON.

The program primarily uses StartedAt (Unix milliseconds) as a fallback date
when no explicit receipt date is present.
*/
type llmRunMetadata struct {
	ResponseID       string `json:"response_id"`
	ResponseLogsURL  string `json:"response_logs_url"`
	Model            string `json:"model"`
	ModelSnapshot    string `json:"model_snapshot"`
	Status           string `json:"status"`
	ReasoningEffort  string `json:"reasoning_effort"`
	Temperature      int    `json:"temperature"`
	TokensIn         int    `json:"tokens_in"`
	TokensCached     int    `json:"tokens_cached"`
	TokensOut        int    `json:"tokens_out"`
	TokensReasoning  int    `json:"tokens_reasoning"`
	TokensTotal      int    `json:"tokens_total"`
	StartedAtUnixMs  int64  `json:"started_at"`
	FinishedAtUnixMs int64  `json:"finished_at"`
	ElapsedMs        int64  `json:"elapsed"`
}

/*
receiptItem represents a single line item from the receipt.
*/
type receiptItem struct {
	LineIndex           int     `json:"line_index"`
	RawLine             string  `json:"raw_line"`
	OriginalProductName string  `json:"original_product_name"`
	ProductNameEnglish  string  `json:"product_name_english"`
	Quantity            float32 `json:"quantity"`
	UnitPrice           int64   `json:"unit_price"`
	LineTotal           int64   `json:"line_total"`
	CategoryKey         string  `json:"category_key"`
}

/*
receiptTotals represents totals calculated by the pipeline.
*/
type receiptTotals struct {
	ReceiptTotal       int64  `json:"receipt_total"`
	ComputedItemsTotal int64  `json:"computed_items_total"`
	TotalCheckMessage  string `json:"total_check_message"`
}

/*
reportOptions controls which receipts are included and where output is written.
*/
type reportOptions struct {
	OutDir      string     `json:"out_dir"`
	Year        int        `json:"year"`
	Month       time.Month `json:"month"`
	OutputPath  string     `json:"output_path"`
	Timezone    string     `json:"timezone"`
	MaxRows     int        `json:"max_rows"`
	ReportTitle string     `json:"report_title"`
}

/*
categoryAgg accumulates spend for a category across many receipts.
*/
type categoryAgg struct {
	Key             string `json:"key"`
	DisplayName     string `json:"display_name"`
	Amount          int64  `json:"amount"`
	ItemLineCount   int64  `json:"item_line_count"`
	ReceiptHitCount int64  `json:"receipt_hit_count"`
}

/*
categoryRow is a rendered row in the final report.
*/
type categoryRow struct {
	Key         string  `json:"key"`
	DisplayName string  `json:"display_name"`
	Amount      int64   `json:"amount"`
	Percent     float64 `json:"percent"`
	Color       string  `json:"color"`
	BarPercent  int     `json:"bar_percent"`
}

/*
monthlyReport is the computed summary for the HTML report.
*/
type monthlyReport struct {
	Title                 string        `json:"title"`
	Year                  int           `json:"year"`
	Month                 time.Month    `json:"month"`
	Timezone              string        `json:"timezone"`
	PeriodStart           time.Time     `json:"period_start"`
	PeriodEnd             time.Time     `json:"period_end"`
	GeneratedAt           time.Time     `json:"generated_at"`
	ReceiptCount          int           `json:"receipt_count"`
	TotalSpent            int64         `json:"total_spent"`
	TotalSpentSourceLabel string        `json:"total_spent_source_label"`
	Rows                  []categoryRow `json:"rows"`
	Notes                 []string      `json:"notes"`
}

/*
main is the CLI entry point.

Example:

	go run . -out ./out -year 2025 -month 12 -o ./report-2025-12.html
*/
func main() {
	options := parseFlags()

	tl.Log(tl.Notice, palette.BlueBold, "Generating monthly expense report for %04s-%02s from '%s'", options.Year, int(options.Month), options.OutDir)

	report, reportErr := buildMonthlyReport(options)
	if reportErr != nil {
		reportErr.QuitIf(xerr.ErrorTypeError)
	}

	htmlText, htmlErr := renderHTML(report)
	if htmlErr != nil {
		htmlErr.QuitIf(xerr.ErrorTypeError)
	}

	writeErr := os.WriteFile(options.OutputPath, []byte(htmlText), 0o644)
	xerr.QuitIfError(writeErr, "write HTML report file")

	tl.Log(tl.Info1, palette.Green, "Saved report to '%s'", options.OutputPath)
}

/*
parseFlags parses CLI flags and returns validated reportOptions.

Defaults:
- current month/year in the selected timezone
- output path: ./report-YYYY-MM.html
*/
func parseFlags() reportOptions {
	outDirFlag := flag.String("out", "./out", "Directory to scan recursively for JSON receipt files")
	yearFlag := flag.Int("year", 0, "Year to report (default: current year)")
	monthFlag := flag.Int("month", 0, "Month to report 1-12 (default: current month)")
	outputFlag := flag.String("o", "", "Output HTML path (default: ./report-YYYY-MM.html)")
	timezoneFlag := flag.String("tz", "America/Bogota", "IANA timezone (e.g., America/Bogota)")
	maxRowsFlag := flag.Int("max-rows", 10, "Maximum category rows before grouping remainder into 'Other'")
	titleFlag := flag.String("title", "", "Report title (default: Expense report — Month Year)")

	flag.Parse()

	location, locationErr := time.LoadLocation(*timezoneFlag)
	if locationErr != nil {
		tl.Log(tl.Warning, palette.PurpleBright, "Invalid timezone '%s'; falling back to UTC", *timezoneFlag)
		location = time.UTC
	}

	now := time.Now().In(location)

	yearValue := *yearFlag
	if yearValue == 0 {
		yearValue = now.Year()
	}

	monthValue := *monthFlag
	if monthValue == 0 {
		monthValue = int(now.Month())
	}
	if monthValue < 1 {
		monthValue = 1
	}
	if monthValue > 12 {
		monthValue = 12
	}

	outputPath := *outputFlag
	if outputPath == "" {
		outputPath = fmt.Sprintf("./tmp/report-%04d-%02d.html", yearValue, monthValue)
	}

	reportTitle := *titleFlag
	if reportTitle == "" {
		monthName := time.Month(monthValue).String()
		reportTitle = fmt.Sprintf("Expense report — %s %d", monthName, yearValue)
	}

	options := reportOptions{
		OutDir:      *outDirFlag,
		Year:        yearValue,
		Month:       time.Month(monthValue),
		OutputPath:  outputPath,
		Timezone:    *timezoneFlag,
		MaxRows:     *maxRowsFlag,
		ReportTitle: reportTitle,
	}

	return options
}

/*
buildMonthlyReport scans JSON files, filters by the selected month/year,
aggregates totals by category_key, and returns a monthlyReport.

Filtering uses a "best available" date:
- receipt_datetime (if present)
- receipt_date (if present)
- llm_run_metadata.started_at (Unix ms)
*/
func buildMonthlyReport(options reportOptions) (report monthlyReport, e *xerr.Error) {
	location, locationErr := time.LoadLocation(options.Timezone)
	if locationErr != nil {
		location = time.UTC
	}

	periodStart := time.Date(options.Year, options.Month, 1, 0, 0, 0, 0, location)
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	jsonPaths, scanErr := collectJSONFiles(options.OutDir)
	if scanErr != nil {
		e = scanErr
		return report, e
	}

	tl.Log(tl.Info1, palette.Cyan, "Found %s JSON files under '%s'", formatIntHuman(int64(len(jsonPaths))), options.OutDir)

	categoryAggByKey := make(map[string]*categoryAgg)
	receiptCount := 0
	totalSpent := int64(0)
	totalSpentFrom := "receipt_total when available, else sum(items.line_total)"

	dateFallbackCount := 0
	explicitDateCount := 0

	for _, jsonPath := range jsonPaths {
		fmt.Println(jsonPath)
		run, loadErr := loadReceiptRun(jsonPath)
		if loadErr != nil {
			tl.Log(tl.Warning, palette.PurpleBright, "Skipping unreadable JSON '%s': %s", jsonPath, loadErr)
			continue
		}

		runTime, runTimeSource, timeErr := determineReceiptTime(run, location)
		if timeErr != nil {
			tl.Log(tl.Warning, palette.PurpleBright, "Skipping JSON with no usable date '%s': %s", jsonPath, timeErr)
			continue
		}

		if runTimeSource == "llm_run_metadata.started_at" {
			dateFallbackCount += 1
		} else {
			explicitDateCount += 1
		}

		if runTime.Before(periodStart) || runTime.After(periodEnd) {
			continue
		}

		receiptCount += 1

		receiptTotal := chooseReceiptTotal(run)
		totalSpent += receiptTotal

		seenCategoriesInThisReceipt := make(map[string]bool)

		for _, item := range run.Items {
			categoryKey := normalizeCategoryKey(item.CategoryKey)
			if categoryKey == "" {
				categoryKey = "uncategorized"
			}

			agg, exists := categoryAggByKey[categoryKey]
			if !exists {
				agg = &categoryAgg{
					Key:             categoryKey,
					DisplayName:     displayCategoryName(categoryKey),
					Amount:          0,
					ItemLineCount:   0,
					ReceiptHitCount: 0,
				}
				categoryAggByKey[categoryKey] = agg
			}

			agg.Amount += item.LineTotal
			agg.ItemLineCount += 1

			alreadyCounted := seenCategoriesInThisReceipt[categoryKey]
			if !alreadyCounted {
				agg.ReceiptHitCount += 1
				seenCategoriesInThisReceipt[categoryKey] = true
			}
		}
	}

	rows := buildCategoryRows(categoryAggByKey, totalSpent, options.MaxRows)

	notes := make([]string, 0)
	notes = append(notes, fmt.Sprintf("Totals source: %s.", totalSpentFrom))
	notes = append(notes, "Category percentages are computed from sum(items.line_total) divided by the displayed total.")
	if dateFallbackCount > 0 && explicitDateCount == 0 {
		notes = append(notes, "Date filtering used llm_run_metadata.started_at for all receipts (no explicit receipt date fields were found).")
	} else if dateFallbackCount > 0 {
		notes = append(notes, "Some receipts used llm_run_metadata.started_at as the date because receipt_date/receipt_datetime were missing.")
	}

	report = monthlyReport{
		Title:                 options.ReportTitle,
		Year:                  options.Year,
		Month:                 options.Month,
		Timezone:              options.Timezone,
		PeriodStart:           periodStart,
		PeriodEnd:             periodEnd,
		GeneratedAt:           time.Now().In(location),
		ReceiptCount:          receiptCount,
		TotalSpent:            totalSpent,
		TotalSpentSourceLabel: totalSpentFrom,
		Rows:                  rows,
		Notes:                 notes,
	}

	tl.Log(tl.Info1, palette.Green, "Included %s receipts for %s-%s", formatIntHuman(int64(receiptCount)), options.Year, int(options.Month))

	return report, e
}

/*
collectJSONFiles recursively walks outDir and returns all *.json file paths.
*/
func collectJSONFiles(outDir string) (paths []string, e *xerr.Error) {
	paths = make([]string, 0)

	walkErr := filepath.WalkDir(outDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), "receipt-analysis.json") {
			paths = append(paths, path)
		}
		return nil
	})
	if walkErr != nil {
		e = xerr.NewErrorEC(walkErr, "walk out directory", "outDir", outDir, false)
		return paths, e
	}

	return paths, e
}

/*
loadReceiptRun reads and unmarshals a receiptRun from JSON.

If the JSON doesn't match the expected shape, it returns an error and the caller can skip it.
*/
func loadReceiptRun(jsonPath string) (run receiptRun, e *xerr.Error) {
	bytesRead, readErr := os.ReadFile(jsonPath)
	if readErr != nil {
		e = xerr.NewErrorEC(readErr, "read JSON file", "path", jsonPath, false)
		return run, e
	}

	unmarshalErr := json.Unmarshal(bytesRead, &run)
	if unmarshalErr != nil {
		e = xerr.NewErrorEC(unmarshalErr, "unmarshal receipt JSON", "path", jsonPath, false)
		return run, e
	}

	return run, e
}

/*
determineReceiptTime finds the best available timestamp to use for filtering.

It returns:
- the chosen time
- a short source label for diagnostics
- a *xerr.Error if no usable time is found
*/
func determineReceiptTime(run receiptRun, location *time.Location) (receiptTime time.Time, source string, e *xerr.Error) {
	if run.ReceiptDateTime != "" {
		parsed, ok := parseReceiptDateTime(run.ReceiptDateTime, location)
		if ok {
			return parsed, "receipt_datetime", e
		}
	}

	if run.ReceiptDate != "" {
		parsed, ok := parseReceiptDate(run.ReceiptDate, location)
		if ok {
			return parsed, "receipt_date", e
		}
	}

	if run.LLMRunMetadata.StartedAtUnixMs > 0 {
		receiptTime = time.UnixMilli(run.LLMRunMetadata.StartedAtUnixMs).In(location)
		return receiptTime, "llm_run_metadata.started_at", e
	}

	e = xerr.NewErrorECOL(fmt.Errorf("no usable date fields present"), "determine receipt time", "hint", "expected receipt_datetime, receipt_date, or llm_run_metadata.started_at")
	return receiptTime, source, e
}

/*
parseReceiptDateTime tries common datetime formats and returns (time, ok).
*/
func parseReceiptDateTime(raw string, location *time.Location) (parsed time.Time, ok bool) {
	candidates := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
	}

	for _, layout := range candidates {
		value, parseErr := time.ParseInLocation(layout, raw, location)
		if parseErr == nil {
			return value, true
		}
	}

	return parsed, false
}

/*
parseReceiptDate tries common date-only formats and returns (time, ok).

The returned time is at 12:00 local time to avoid edge cases around DST boundaries.
*/
func parseReceiptDate(raw string, location *time.Location) (parsed time.Time, ok bool) {
	candidates := []string{
		"2006-01-02",
		"02/01/2006",
		"2006/01/02",
	}

	for _, layout := range candidates {
		value, parseErr := time.ParseInLocation(layout, raw, location)
		if parseErr == nil {
			return time.Date(value.Year(), value.Month(), value.Day(), 12, 0, 0, 0, location), true
		}
	}

	return parsed, false
}

/*
chooseReceiptTotal selects the overall total for a receipt.

Preference:
1) totals.receipt_total if > 0
2) totals.computed_items_total if > 0
3) sum(items.line_total)
*/
func chooseReceiptTotal(run receiptRun) int64 {
	if run.Totals.ReceiptTotal > 0 {
		return run.Totals.ReceiptTotal
	}
	if run.Totals.ComputedItemsTotal > 0 {
		return run.Totals.ComputedItemsTotal
	}

	sum := int64(0)
	for _, item := range run.Items {
		sum += item.LineTotal
	}
	return sum
}

/*
buildCategoryRows converts aggregations into sorted rows, assigns colors, and optionally groups overflow into "Other".
*/
func buildCategoryRows(categoryAggByKey map[string]*categoryAgg, totalSpent int64, maxRows int) []categoryRow {
	rows := make([]categoryRow, 0, len(categoryAggByKey))

	for _, agg := range categoryAggByKey {
		percent := 0.0
		if totalSpent > 0 {
			percent = (float64(agg.Amount) / float64(totalSpent)) * 100.0
		}

		barPercent := int(math.Round(percent))
		if agg.Amount > 0 && barPercent == 0 {
			barPercent = 1
		}
		if barPercent > 100 {
			barPercent = 100
		}

		row := categoryRow{
			Key:         agg.Key,
			DisplayName: agg.DisplayName,
			Amount:      agg.Amount,
			Percent:     percent,
			Color:       "",
			BarPercent:  barPercent,
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(firstIndex int, secondIndex int) bool {
		return rows[firstIndex].Amount > rows[secondIndex].Amount
	})

	if maxRows < 3 {
		maxRows = 3
	}

	if len(rows) > maxRows {
		keep := rows[:maxRows-1]
		rest := rows[maxRows-1:]

		otherAmount := int64(0)
		for _, row := range rest {
			otherAmount += row.Amount
		}

		otherPercent := 0.0
		if totalSpent > 0 {
			otherPercent = (float64(otherAmount) / float64(totalSpent)) * 100.0
		}

		otherBarPercent := int(math.Round(otherPercent))
		if otherAmount > 0 && otherBarPercent == 0 {
			otherBarPercent = 1
		}
		if otherBarPercent > 100 {
			otherBarPercent = 100
		}

		other := categoryRow{
			Key:         "other",
			DisplayName: "Other",
			Amount:      otherAmount,
			Percent:     otherPercent,
			Color:       "",
			BarPercent:  otherBarPercent,
		}

		rows = append(keep, other)
	}

	paletteColors := []string{
		"#2563EB", "#7C3AED", "#059669", "#DB2777", "#D97706",
		"#0EA5E9", "#65A30D", "#9333EA", "#F43F5E", "#14B8A6",
		"#4F46E5", "#B45309",
	}

	for index := 0; index < len(rows); index += 1 {
		color := paletteColors[index%len(paletteColors)]
		rows[index].Color = color
	}

	return rows
}

/*
normalizeCategoryKey trims and normalizes a category key for consistent grouping.
*/
func normalizeCategoryKey(categoryKey string) string {
	trimmed := strings.TrimSpace(categoryKey)
	trimmed = strings.ToLower(trimmed)
	return trimmed
}

/*
displayCategoryName maps known keys to nicer names and falls back to a title-cased variant.
*/
func displayCategoryName(categoryKey string) string {
	known := map[string]string{
		"personal_care":      "Personal care",
		"household_cleaning": "Household cleaning",
		"drinks_soft":        "Drinks (non-alcoholic)",
		"bakery":             "Bakery",
		"other_food":         "Other food",
		"other":              "Other",
		"uncategorized":      "Uncategorized",
	}

	name, exists := known[categoryKey]
	if exists {
		return name
	}

	parts := strings.Split(categoryKey, "_")
	for index := 0; index < len(parts); index += 1 {
		part := parts[index]
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

/*
renderHTML converts a monthlyReport into a single HTML string using inline CSS only.
*/
func renderHTML(report monthlyReport) (htmlText string, e *xerr.Error) {
	var buffer bytes.Buffer

	totalFormatted := formatCOP(report.TotalSpent)
	monthName := report.Month.String()

	buffer.WriteString("<!doctype html>")
	buffer.WriteString("<html>")
	buffer.WriteString("<head>")
	buffer.WriteString(`<meta charset="utf-8">`)
	buffer.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	buffer.WriteString("</head>")

	bodyStyle := "margin:0;padding:0;background-color:#F3F4F6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Inter,Arial,sans-serif;color:#111827;"
	buffer.WriteString(`<body style="` + bodyStyle + `">`)

	// Outer wrapper table (email-safe centering).
	buffer.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="border-collapse:collapse;background-color:#F3F4F6;">`)
	buffer.WriteString(`<tr>`)
	buffer.WriteString(`<td align="center" style="padding:24px;">`)

	// Main container.
	buffer.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="680" style="border-collapse:separate;background-color:#F3F4F6;width:680px;max-width:680px;">`)
	buffer.WriteString(`<tr><td style="padding:0;">`)

	// Header.
	buffer.WriteString(`<div style="padding:8px 4px 18px 4px;">`)
	buffer.WriteString(`<div style="font-size:24px;font-weight:800;line-height:1.2;color:#111827;">` + html.EscapeString(report.Title) + `</div>`)
	buffer.WriteString(`<div style="margin-top:6px;font-size:13px;line-height:1.5;color:#6B7280;">`)
	buffer.WriteString(`Period: <span style="font-weight:700;color:#111827;">` + html.EscapeString(monthName) + ` ` + strconv.Itoa(report.Year) + `</span>`)
	buffer.WriteString(` &nbsp;•&nbsp; Receipts: <span style="font-weight:700;color:#111827;">` + formatIntHuman(int64(report.ReceiptCount)) + `</span>`)
	buffer.WriteString(` &nbsp;•&nbsp; Timezone: <span style="font-weight:700;color:#111827;">` + html.EscapeString(report.Timezone) + `</span>`)
	buffer.WriteString(`</div>`)
	buffer.WriteString(`</div>`)

	// Summary card.
	buffer.WriteString(cardOpen())
	buffer.WriteString(`<div style="padding:18px 18px 6px 18px;">`)
	buffer.WriteString(`<div style="font-size:12px;letter-spacing:0.10em;text-transform:uppercase;color:#6B7280;">Total spent</div>`)
	buffer.WriteString(`<div style="margin-top:6px;font-size:34px;font-weight:900;line-height:1.1;color:#111827;">` + html.EscapeString(totalFormatted) + `</div>`)
	buffer.WriteString(`<div style="margin-top:8px;font-size:13px;line-height:1.5;color:#6B7280;">`)
	buffer.WriteString(`From <span style="font-weight:700;color:#111827;">` + report.PeriodStart.Format("2006-01-02") + `</span> to <span style="font-weight:700;color:#111827;">` + report.PeriodEnd.Format("2006-01-02") + `</span>`)
	buffer.WriteString(`</div>`)
	buffer.WriteString(`</div>`)

	buffer.WriteString(`<div style="padding:0 18px 18px 18px;">`)
	buffer.WriteString(`<div style="height:1px;background-color:#E5E7EB;width:100%;"></div>`)
	buffer.WriteString(`<div style="margin-top:14px;font-size:14px;font-weight:800;color:#111827;">Category breakdown</div>`)
	buffer.WriteString(`<div style="margin-top:4px;font-size:12px;line-height:1.5;color:#6B7280;">Percent of total spend for the month.</div>`)
	buffer.WriteString(`</div>`)

	// Category table.
	buffer.WriteString(`<div style="padding:0 18px 18px 18px;">`)
	if report.ReceiptCount == 0 || len(report.Rows) == 0 {
		buffer.WriteString(`<div style="padding:14px;border:1px dashed #D1D5DB;border-radius:12px;background-color:#FAFAFA;color:#6B7280;font-size:13px;line-height:1.6;">`)
		buffer.WriteString(`No receipts found for this month in the selected directory.`)
		buffer.WriteString(`</div>`)
	} else {
		buffer.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="border-collapse:separate;border-spacing:0 10px;">`)
		for _, row := range report.Rows {
			buffer.WriteString(`<tr>`)
			buffer.WriteString(`<td style="padding:12px 12px 12px 12px;background-color:#FFFFFF;border:1px solid #E5E7EB;border-radius:12px;">`)

			// Row header.
			buffer.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="border-collapse:collapse;">`)
			buffer.WriteString(`<tr>`)

			// Category name with dot.
			buffer.WriteString(`<td style="vertical-align:top;padding-right:10px;">`)
			buffer.WriteString(`<div style="display:inline-block;width:10px;height:10px;border-radius:999px;background-color:` + row.Color + `;margin-right:8px;position:relative;top:1px;"></div>`)
			buffer.WriteString(`<span style="font-size:14px;font-weight:800;color:#111827;">` + html.EscapeString(row.DisplayName) + `</span>`)
			buffer.WriteString(`</td>`)

			// Amount.
			buffer.WriteString(`<td align="right" style="vertical-align:top;">`)
			buffer.WriteString(`<div style="font-size:14px;font-weight:900;color:#111827;">` + html.EscapeString(formatCOP(row.Amount)) + `</div>`)
			buffer.WriteString(`<div style="margin-top:2px;font-size:12px;font-weight:800;color:#6B7280;">` + fmt.Sprintf("%.1f%%", row.Percent) + `</div>`)
			buffer.WriteString(`</td>`)

			buffer.WriteString(`</tr>`)

			// Bar.
			buffer.WriteString(`<tr><td colspan="2" style="padding-top:10px;">`)
			buffer.WriteString(`<div style="width:100%;height:10px;border-radius:999px;background-color:#EEF2FF;overflow:hidden;border:1px solid #E5E7EB;">`)
			buffer.WriteString(`<div style="height:10px;width:` + strconv.Itoa(row.BarPercent) + `%;background-color:` + row.Color + `;border-radius:999px;"></div>`)
			buffer.WriteString(`</div>`)
			buffer.WriteString(`</td></tr>`)

			buffer.WriteString(`</table>`)

			buffer.WriteString(`</td>`)
			buffer.WriteString(`</tr>`)
		}
		buffer.WriteString(`</table>`)
	}
	buffer.WriteString(`</div>`)

	// Notes card.
	buffer.WriteString(`<div style="padding:0 0 18px 0;">`)
	buffer.WriteString(cardOpen())
	buffer.WriteString(`<div style="padding:16px 18px 16px 18px;">`)
	buffer.WriteString(`<div style="font-size:13px;font-weight:900;color:#111827;">Notes</div>`)
	buffer.WriteString(`<div style="margin-top:10px;font-size:12px;line-height:1.7;color:#6B7280;">`)
	for _, note := range report.Notes {
		buffer.WriteString(`• ` + html.EscapeString(note) + `<br>`)
	}
	buffer.WriteString(`</div>`)
	buffer.WriteString(`<div style="margin-top:12px;font-size:11px;color:#9CA3AF;">Generated ` + html.EscapeString(report.GeneratedAt.Format("2006-01-02 15:04:05")) + `</div>`)
	buffer.WriteString(`</div>`)
	buffer.WriteString(cardClose())
	buffer.WriteString(`</div>`)

	// Close main container and wrappers.
	buffer.WriteString(`</td></tr>`)
	buffer.WriteString(`</table>`)

	buffer.WriteString(`</td>`)
	buffer.WriteString(`</tr>`)
	buffer.WriteString(`</table>`)

	buffer.WriteString(`</body>`)
	buffer.WriteString(`</html>`)

	htmlText = buffer.String()
	return htmlText, e
}

/*
cardOpen returns the opening HTML for a card-like container (email-safe).
*/
func cardOpen() string {
	return `<div style="background-color:#FFFFFF;border:1px solid #E5E7EB;border-radius:16px;box-shadow:0 8px 24px rgba(17,24,39,0.06);overflow:hidden;">`
}

/*
cardClose returns the closing HTML for a card-like container.
*/
func cardClose() string {
	return `</div>`
}

/*
formatCOP formats an integer amount as COP with dot thousand separators.

Example:

	71630 -> "COP 71.630"
*/
func formatCOP(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	raw := strconv.FormatInt(amount, 10)
	grouped := groupThousands(raw, ".")
	return fmt.Sprintf("%sCOP %s", sign, grouped)
}

/*
groupThousands groups digits in a base-10 string using the provided separator.
*/
func groupThousands(raw string, sep string) string {
	if len(raw) <= 3 {
		return raw
	}

	var builder strings.Builder
	firstGroupLen := len(raw) % 3
	if firstGroupLen == 0 {
		firstGroupLen = 3
	}

	builder.WriteString(raw[:firstGroupLen])

	for index := firstGroupLen; index < len(raw); index += 3 {
		builder.WriteString(sep)
		builder.WriteString(raw[index : index+3])
	}

	return builder.String()
}

/*
formatIntHuman formats a count with comma separators for readability.
*/
func formatIntHuman(value int64) string {
	raw := strconv.FormatInt(value, 10)
	return groupThousands(raw, ",")
}
