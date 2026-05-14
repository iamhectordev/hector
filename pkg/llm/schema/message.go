package schema

// Role identifies who sent a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// FinishReason is why the model stopped generating.
type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"
	FinishReasonToolCalls FinishReason = "tool_calls"
	FinishReasonLength    FinishReason = "length"
)

// Message is one turn in a model-facing conversation.
type Message struct {
	Role         Role
	Content      string
	ToolCalls    []ToolCall
	ToolCallID   string
	FinishReason FinishReason
}

// SystemMessage returns a system-role message.
func SystemMessage(content string) *Message {
	return &Message{Role: RoleSystem, Content: content}
}

// UserMessage returns a user-role message.
func UserMessage(content string) *Message {
	return &Message{Role: RoleUser, Content: content}
}

// AssistantMessage returns an assistant-role message.
func AssistantMessage(content string) *Message {
	return &Message{Role: RoleAssistant, Content: content}
}

// ToolResultMessage returns a tool-role message for a previous tool call.
func ToolResultMessage(callID, content string) *Message {
	return &Message{Role: RoleTool, ToolCallID: callID, Content: content}
}
