package message

// RoleType identifies who sent a chat message.
type RoleType string

const (
	User      RoleType = "user"
	Assistant RoleType = "assistant"
)

// Message is one turn in a conversation.
type Message struct {
	Role    RoleType
	Content string
}

// UserMessage returns a user-role message.
func UserMessage(content string) *Message {
	return &Message{Role: User, Content: content}
}

// AssistantMessage returns an assistant-role message.
func AssistantMessage(content string) *Message {
	return &Message{Role: Assistant, Content: content}
}
