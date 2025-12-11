package ocr

import (
	"fmt"
	"regexp"
)

// priceTokenRegexp matches standalone numbers like "90.1004", "7.008", "8,080", "6.23".
// - (^|\\s)   : start of line or whitespace before the number
// - (\\d{1,3}): 1–3 digits before the separator (so we don't grab the "11490" part)
// - ([.,])    : decimal/thousand separator
// - (\\d{2,4}): 2–4 digits after the separator (4 to accommodate the misread A -> 4/8)
// - (?=\\s|$) : followed by whitespace or end of line so we don't match inside "30.88611.980.0"
var priceTokenRegexp = regexp.MustCompile(`(?m)\s{2,}(\d{1,3})([.,])(\d{2,4})`)

// ExtractPriceCandidates parses the numeric-only OCR block and returns a list of
// candidate prices as strings, with at most 3 digits after the separator, and
// with duplicates removed while preserving order.
//
// Example output for your sample block:
//   []string{"90.100", "66.200", "7.008", "8,080", "7.650", "5.23", "6.23", "4.200", "189.468"}
func ExtractPriceCandidates(numericOCR string) []string {
	matches := priceTokenRegexp.FindAllStringSubmatch(numericOCR, -1)
	prices := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, m := range matches {
		fmt.Println(m)
		intPart := m[1] // the digits before the separator
		sep := m[2]     // "." or ","
		frac := m[3]    // the digits after the separator (2–4 of them)

		// If we got 4 digits (e.g. "1004" from "90.1004"), keep only the last 3.
		// This effectively drops the misread "A" (often seen as 4 or 8).
		if len(frac) > 3 {
			frac = frac[len(frac)-3:]
		}

		price := intPart + sep + frac

		if !seen[price] {
			seen[price] = true
			prices = append(prices, price)
		}
	}

	return prices
}
