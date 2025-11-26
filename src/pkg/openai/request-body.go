package openai

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

/*
Get body of http.Response, handle compression.
In case of error - return error message instead.
Pass original url for more clear logging.
*/
func GetBody(resp *http.Response, urlStr string) (body []byte, e *xerr.Error) {
	var reader io.ReadCloser
	contentEncoding := resp.Header.Get("Content-Encoding")

	tl.Log(tl.Verbose5, palette.BlueDim, "Get body (content encoding is '%s') for '%s'", contentEncoding, urlStr)
	switch contentEncoding {
	case "gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return body, xerr.NewError(err, "Unable to get gzip reader", urlStr)
		}
		defer reader.Close()
	case "deflate":
		reader = flate.NewReader(resp.Body)
		defer reader.Close()
	case "br":
		reader = io.NopCloser(brotli.NewReader(resp.Body)) // Wrap brotli.Reader to satisfy io.ReadCloser
		// no need to close brotli reader
	case "", "none":
		// No compression, just use the response body as-is
		reader = resp.Body
	default:
		// No compression, just use the response body as-is
		reader = resp.Body
		tl.Log(tl.Warning, palette.YellowDim, "\nUnsupported %s: '%s'", "Content-Encoding", contentEncoding)
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return body, xerr.NewError(err, "Failed to read response body", urlStr)
	}
	tl.Log(tl.Verbose6, palette.GreenDim, "Got body length %s (content encoding is '%s') for '%s'", len(body), contentEncoding, urlStr)

	return body, nil
}
