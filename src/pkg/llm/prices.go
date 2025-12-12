package llm


import (
	"encoding/json"
	"os"
	"strings"

	"github.com/tuumbleweed/xerr"
)

/*
parseOcrPricesJSON parses a JSON array of strings (e.g. ["7.008","8,080"])
into a []string.

It returns a *xerr.Error on failure.
*/
func parseOcrPricesJSON(ocrPricesJSON string) (prices []string, e *xerr.Error) {
	trimmedJSON := strings.TrimSpace(ocrPricesJSON)

	parseErr := json.Unmarshal([]byte(trimmedJSON), &prices)
	if parseErr != nil {
		e = xerr.NewError(parseErr, "parse OCR prices JSON", "json.Unmarshal")
		return prices, e
	}

	return prices, e
}

/*
readOcrPricesFromFile reads a JSON array of strings from pricesPath and returns it
as a []string.

It returns a *xerr.Error on failure.
*/
func ReadOcrPricesFromFile(pricesPath string) (prices []string, e *xerr.Error) {
	var fileBytes []byte
	var readErr error

	fileBytes, readErr = os.ReadFile(pricesPath)
	if readErr != nil {
		e = xerr.NewError(readErr, "read OCR prices file", pricesPath)
		return prices, e
	}

	ocrPricesJSON := string(fileBytes)

	prices, e = parseOcrPricesJSON(ocrPricesJSON)
	if e != nil {
		// Add file context (so errors clearly point to the path that failed
		return prices, e
	}

	return prices, e
}
