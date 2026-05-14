package schema

// CompletionRequest is the provider-facing request for one model completion.
type CompletionRequest struct {
	System   string
	Messages []*Message
	Tools    []ToolDefinition
}
