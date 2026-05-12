package schema

// CompletionRequest is the provider-facing request for one model completion.
type CompletionRequest struct {
	Messages []*Message
	Tools    []ToolDefinition
}
