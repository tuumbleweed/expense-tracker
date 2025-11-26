package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"this-project/src/pkg/util"
	"time"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

const (
	CreateResponseTimeout = 300 * time.Second // model may take a while
	GetResponseTimeout    = 30 * time.Second  // metadata fetch should be fast
	FileUploadTimeout     = 60 * time.Second
)

/*
This file contains a tiny, dependency-free REST client for the OpenAI Responses API.

Key pieces:
- POST /v1/responses (createResponse): synchronous or may return an in-progress response
- GET  /v1/responses/{id} (getResponseByID): fetch status/output/usage
- Optional file upload helper (UploadUserFile) via /v1/files
*/

const OpenAIAPIURL = "https://api.openai.com/v1"

/*
createResponse performs POST /v1/responses and returns the parsed response object.
It may return a "completed" response immediately, or an "in_progress" one (future-friendly).
*/
func createResponse(apiKey string, payload requestPayload) (response responseObject, e *xerr.Error) {
	tl.Log(tl.Info, palette.Blue, "%s %s to '%s'", "Creating", "response", OpenAIAPIURL+"/responses")

	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return responseObject{}, xerr.NewError(marshalErr, "Failed to marshal request payload", payload)
	}

	url := fmt.Sprintf("%s/responses", OpenAIAPIURL)
	req, newReqErr := http.NewRequest("POST", url, bytes.NewBuffer(encoded))
	if newReqErr != nil {
		return responseObject{}, xerr.NewError(newReqErr, "Failed to create HTTP request", nil)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: CreateResponseTimeout}
	resp, httpErr := client.Do(req)
	if httpErr != nil {
		return responseObject{}, xerr.NewError(httpErr, "HTTP error during createResponse", map[string]any{"url": url})
	}
	defer resp.Body.Close()

	respBody, e := GetBody(resp, resp.Request.URL.String())
	if e != nil {
		return responseObject{}, e
	}
	if resp.StatusCode != http.StatusOK {
		return responseObject{}, xerr.NewError(fmt.Errorf("status is '%s'", resp.Status), "API error from /v1/responses", string(respBody))
	}
	tl.LogJSON(tl.Debug, palette.CyanDim, "openai response body", respBody)

	var parsed responseObject
	decodeErr := json.Unmarshal(respBody, &parsed)
	if decodeErr != nil {
		return responseObject{}, xerr.NewError(decodeErr, "Failed to decode response body", nil)
	}

	return parsed, nil
}

/*
getResponseByID performs GET /v1/responses/{id} and returns the parsed response object.
*/
func getResponseByID(apiKey, responseID string) (response responseObject, e *xerr.Error) {
	url := fmt.Sprintf("%s/responses/%s", OpenAIAPIURL, responseID)

	req, newReqErr := http.NewRequest("GET", url, nil)
	if newReqErr != nil {
		return responseObject{}, xerr.NewError(newReqErr, "Failed to create HTTP request", map[string]any{"response_id": responseID})
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: GetResponseTimeout}
	resp, httpErr := client.Do(req)
	if httpErr != nil {
		return responseObject{}, xerr.NewError(httpErr, "HTTP error during getResponseByID", map[string]any{"url": url})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read everything once
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return responseObject{}, xerr.NewError(readErr, "Failed to read response body", nil)
		}
		return responseObject{}, xerr.NewError(fmt.Errorf("status is '%s'", resp.Status), "API error from GET /v1/responses/{id}", string(respBody))
	}

	respBody, e := GetBody(resp, resp.Request.URL.String())
	if e != nil {
		return responseObject{}, e
	}
	if resp.StatusCode != http.StatusOK {
		return responseObject{}, xerr.NewError(fmt.Errorf("status is '%s'", resp.Status), "API error from /v1/responses", string(respBody))
	}
	tl.LogJSON(tl.Debug, palette.CyanDim, "openai response body", respBody)

	var parsed responseObject
	decodeErr := json.Unmarshal(respBody, &parsed)
	if decodeErr != nil {
		return responseObject{}, xerr.NewError(decodeErr, "Failed to decode response body", nil)
	}

	return parsed, nil
}

// ----- Small helpers -----

/*
extractOutputText collects all "output_text" fragments from the response into a single string.
*/
func extractOutputText(resp *responseObject) string {
	var builder bytes.Buffer
	for _, out := range resp.Output {
		if out.Type != "message" {
			continue
		}
		for _, c := range out.Content {
			if c.Type == "output_text" && c.Text != "" {
				_, _ = builder.WriteString(c.Text)
			}
		}
	}
	return builder.String()
}


/*
waitForResponseCompletion polls GET /v1/responses/{id} every interval until terminal state
or until timeout is reached (if timeout > 0). On success, returns the final response object.
On failure/cancel/expire/timeout, returns a *xerr.Error with the API's error payload in Context
(where available) and logs a heartbeat each poll.
*/
func waitForResponseCompletion(apiKey, responseID string, waitInterval, timeout time.Duration) (final responseObject, e *xerr.Error) {
	previousStatus := ""
	poll := 0

	var (
		useTimeout bool
		deadline   time.Time
	)
	if timeout > 0 {
		useTimeout = true
		deadline = time.Now().Add(timeout)
	}

	var lastResp responseObject

	for {
		// Timeout check before each poll
		if useTimeout && time.Now().After(deadline) {
			msg := fmt.Sprintf("Response polling timed out after %s", timeout)
			tl.Log(tl.Info1, palette.Purple, "%s; last known id='%s'", msg, responseID)
			lastResp.Status = "timeout"
			return lastResp, xerr.NewError(fmt.Errorf("timeout"), msg, timeout)
		}

		poll += 1

		resp, getErr := getResponseByID(apiKey, responseID)
		if getErr != nil {
			return lastResp, getErr
		}
		lastResp = resp

		// Always print a heartbeat every poll; also note transitions verbosely.
		if resp.Status != previousStatus {
			tl.Log(tl.Verbose, palette.Cyan, "Response status changed: '%s'", resp.Status)
			previousStatus = resp.Status
		}
		tl.Log(tl.Verbose, palette.Cyan, "Poll #%v: status is '%s'", poll, resp.Status)

		switch resp.Status {
		case "completed", "incomplete", "":
			return resp, nil
		case "failed", "cancelled", "expired":
			msg := fmt.Sprintf("Response ended with status '%s'", resp.Status)
			tl.Log(tl.Info1, palette.Purple, "%s id is '%s'", msg, responseID)
			return resp, xerr.NewError(fmt.Errorf("%s", resp.Status), msg, resp.Error)
		default:
			util.WaitForSeconds(waitInterval.Seconds())
		}
	}
}
