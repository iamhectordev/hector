package schema

// ToolChoice forces the model to call a specific tool instead of choosing freely.
type ToolChoice struct {
	Name string // name of the tool to force-call
}

// CompletionRequest is the provider-facing request for one model completion.
type CompletionRequest struct {
	System     string
	Messages   []*Message
	Tools      []ToolDefinition
	ToolChoice *ToolChoice // nil = model chooses
}
