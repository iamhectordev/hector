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
	Role         Role          `json:"Role"`
	Content      string        `json:"Content,omitempty"`
	Parts        []MessagePart `json:"Parts,omitempty"`
	ToolCalls    []ToolCall    `json:"ToolCalls,omitempty"`
	ToolCallID   string        `json:"ToolCallID,omitempty"`
	FinishReason FinishReason  `json:"FinishReason,omitempty"`
}

// MessagePartType identifies the payload carried by a structured message part.
type MessagePartType string

const (
	// MessagePartTypeText carries plain text content.
	MessagePartTypeText MessagePartType = "text"
	// MessagePartTypeImage carries an image input.
	MessagePartTypeImage MessagePartType = "image"
)

// MessagePart is one ordered content block in a model-facing message.
type MessagePart struct {
	// Type identifies which payload field is active.
	Type MessagePartType `json:"Type"`
	// Text is used when Type is MessagePartTypeText.
	Text string `json:"Text,omitempty"`
	// Image is used when Type is MessagePartTypeImage.
	Image *ImagePart `json:"Image,omitempty"`
}

// ImagePart is an image input stored as base64 plus MIME metadata.
// Providers adapt this provider-neutral representation to their native wire format.
type ImagePart struct {
	// ID links this image payload back to the XML <img id="..."> node.
	ID string `json:"ID,omitempty"`
	// Base64Data is the raw image bytes encoded without a data URL prefix.
	Base64Data string `json:"Base64Data,omitempty"`
	// MIMEType is the image media type, for example "image/png".
	MIMEType string `json:"MIMEType,omitempty"`
	// Detail is a provider-neutral image fidelity hint: "", "auto", "low", or "high".
	Detail string `json:"Detail,omitempty"`
}

// SystemMessage returns a system-role message.
func SystemMessage(content string) *Message {
	return &Message{Role: RoleSystem, Content: content}
}

// UserMessage returns a user-role message.
func UserMessage(content string) *Message {
	return &Message{Role: RoleUser, Content: content}
}

// UserMessageWithParts returns a user-role message with structured multimodal parts.
// Content remains populated as a text fallback and for providers without multimodal support.
func UserMessageWithParts(content string, parts []MessagePart) *Message {
	return &Message{Role: RoleUser, Content: content, Parts: parts}
}

// TextPart returns a text message part.
func TextPart(text string) MessagePart {
	return MessagePart{Type: MessagePartTypeText, Text: text}
}

// NewImagePart returns an image message part linked to the source attachment ID.
func NewImagePart(id, base64Data, mimeType string) MessagePart {
	return MessagePart{
		Type: MessagePartTypeImage,
		Image: &ImagePart{
			ID:         id,
			Base64Data: base64Data,
			MIMEType:   mimeType,
		},
	}
}

// AssistantMessage returns an assistant-role message.
func AssistantMessage(content string) *Message {
	return &Message{Role: RoleAssistant, Content: content}
}

// ToolResultMessage returns a tool-role message for a previous tool call.
func ToolResultMessage(callID, content string) *Message {
	return &Message{Role: RoleTool, ToolCallID: callID, Content: content}
}
