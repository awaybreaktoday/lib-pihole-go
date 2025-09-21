package pihole

import (
	"encoding/json"
	"fmt"
)

type apiErrorDetails struct {
	Key     string      `json:"key"`
	Message string      `json:"message"`
	Hint    interface{} `json:"hint"`
}

type apiErrorPayload struct {
	Error *apiErrorDetails `json:"error"`
}

func parseAPIError(body []byte) (*apiErrorDetails, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	var payload apiErrorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	if payload.Error == nil {
		return nil, fmt.Errorf("missing error payload")
	}

	return payload.Error, nil
}

type DNSAPIError struct {
	StatusCode int
	Key        string
	Message    string
	Hint       interface{}
}

func (e *DNSAPIError) Error() string {
	if e == nil {
		return ""
	}

	if e.Key != "" {
		return fmt.Sprintf("pi-hole DNS API error (%d %s): %s", e.StatusCode, e.Key, e.Message)
	}

	return fmt.Sprintf("pi-hole DNS API error (%d): %s", e.StatusCode, e.Message)
}

type CNAMEAPIError struct {
	StatusCode int
	Key        string
	Message    string
	Hint       interface{}
}

func (e *CNAMEAPIError) Error() string {
	if e == nil {
		return ""
	}

	if e.Key != "" {
		return fmt.Sprintf("pi-hole CNAME API error (%d %s): %s", e.StatusCode, e.Key, e.Message)
	}

	return fmt.Sprintf("pi-hole CNAME API error (%d): %s", e.StatusCode, e.Message)
}

func newDNSAPIError(status int, body []byte) error {
	if details, err := parseAPIError(body); err == nil {
		return &DNSAPIError{StatusCode: status, Key: details.Key, Message: details.Message, Hint: details.Hint}
	}

	return fmt.Errorf("received unexpected status code %d %s", status, string(body))
}

func newCNAMEAPIError(status int, body []byte) error {
	if details, err := parseAPIError(body); err == nil {
		return &CNAMEAPIError{StatusCode: status, Key: details.Key, Message: details.Message, Hint: details.Hint}
	}

	return fmt.Errorf("received unexpected status code %d %s", status, string(body))
}
