package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const apiErrorSummaryBodyWidth = 160

// APIError retains the safe, user-visible context for a request that reached
// the API boundary but did not produce a usable documented response.
type APIError struct {
	OperationID string
	Method      string
	URL         string
	Status      int
	Body        string
	Cause       error
	Kind        string
	Reason      string
	Code        string
}

func (apiError *APIError) Error() string {
	if apiError == nil {
		return "API request failed"
	}
	if apiError.Reason != "" {
		return SanitizeCell(apiError.Reason)
	}
	prefix := strings.TrimSpace(apiError.OperationID)
	if prefix == "" {
		prefix = "API request"
	}
	if apiError.Status > 0 {
		summary := fmt.Sprintf("%s returned HTTP %d", prefix, apiError.Status)
		if status := http.StatusText(apiError.Status); status != "" {
			summary += " " + status
		}
		if body := SanitizeCell(apiError.Body); body != "" {
			summary += ": " + ansi.Truncate(body, apiErrorSummaryBodyWidth, "…")
		} else if apiError.Cause != nil {
			summary += ": " + SanitizeCell(apiError.Cause.Error())
		}
		return summary
	}
	if apiError.Cause != nil {
		return prefix + " failed: " + SanitizeCell(apiError.Cause.Error())
	}
	return prefix + " failed"
}

func (apiError *APIError) DialogTitle() string {
	if apiError != nil && apiError.Kind == "Error" && apiError.Reason != "" {
		return "Error"
	}
	return "API Error"
}

func (apiError *APIError) DialogMessage() string {
	if apiError != nil && apiError.Reason != "" {
		return SanitizeCell(apiError.Reason)
	}
	return apiError.Error()
}

func (apiError *APIError) DialogContext() string {
	if apiError == nil {
		return ""
	}
	var parts []string
	if apiError.Code != "" {
		parts = append(parts, SanitizeCell(apiError.Code))
	}
	if apiError.Status > 0 {
		status := fmt.Sprintf("HTTP %d", apiError.Status)
		if reason := http.StatusText(apiError.Status); reason != "" {
			status += " " + reason
		}
		parts = append(parts, status)
	}
	return strings.Join(parts, " · ")
}

func (apiError *APIError) Details() string {
	if apiError == nil {
		return "API request failed"
	}
	lines := []string{"Operation: " + SanitizeCell(apiError.OperationID)}
	request := strings.TrimSpace(SanitizeCell(apiError.Method) + " " + SanitizeCell(apiError.URL))
	if request != "" {
		lines = append(lines, "Request: "+request)
	}
	if apiError.Status > 0 {
		status := fmt.Sprintf("%d", apiError.Status)
		if reason := http.StatusText(apiError.Status); reason != "" {
			status += " " + reason
		}
		lines = append(lines, "Status: "+status)
	}
	if apiError.Cause != nil {
		lines = append(lines, "Cause: "+Sanitize(apiError.Cause.Error()))
	}
	if apiError.Body != "" {
		lines = append(lines, "", "Response body:", Sanitize(apiError.Body))
	}
	return strings.Join(lines, "\n")
}

func newAPIError(operationID string, request *http.Request, response *http.Response, body []byte, cause error) *APIError {
	apiError := &APIError{OperationID: operationID, Body: string(body), Cause: cause}
	if request != nil {
		apiError.Method = request.Method
		apiError.URL = safeErrorURL(request.URL)
	}
	if response != nil {
		apiError.Status = response.StatusCode
	}
	var envelope struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
		Code   string `json:"code"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Kind == "Error" && SanitizeCell(envelope.Reason) != "" {
		apiError.Kind = envelope.Kind
		apiError.Reason = envelope.Reason
		apiError.Code = envelope.Code
	}
	return apiError
}

func safeErrorURL(requestURL *url.URL) string {
	if requestURL == nil {
		return ""
	}
	safe := *requestURL
	safe.User = nil
	safe.RawQuery = ""
	safe.ForceQuery = false
	safe.Fragment = ""
	return safe.String()
}
