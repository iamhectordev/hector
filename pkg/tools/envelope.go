package tools

import "encoding/json"

// Envelope is the standard output wrapper for typed tools.
type Envelope[O any] struct {
	Status  string `json:"status"`
	Result  *O     `json:"result,omitempty"`
	Message string `json:"message,omitempty"`
}

// OK serialises a successful tool result into an envelope string.
func OK[O any](result O) (string, error) {
	b, err := json.Marshal(Envelope[O]{Status: "ok", Result: &result})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type errorEnvelope = Envelope[struct{}]

// Fail serialises an error message into an envelope string.
func Fail(message string) (string, error) {
	b, err := json.Marshal(errorEnvelope{Status: "error", Message: message})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
