package gateway

import (
	"time"

	"github.com/iamhectordev/hector/internal/ulid"
	"github.com/iamhectordev/hector/pkg/waffle"
)

var _ waffle.Event[string] = (*MessageReceivedEvent)(nil)

// MessageReceivedEvent records that the gateway accepted an inbound message.
type MessageReceivedEvent struct {
	id         string
	occurredAt time.Time
	data       string
}

// NewMessageReceivedEvent creates a gateway message-received event.
func NewMessageReceivedEvent(data string) *MessageReceivedEvent {
	return &MessageReceivedEvent{
		id:         ulid.New("evt"),
		occurredAt: time.Now().UTC(),
		data:       data,
	}
}

func (e *MessageReceivedEvent) ID() string {
	return e.id
}

func (e *MessageReceivedEvent) Type() string {
	return "gateway.message_received"
}

func (e *MessageReceivedEvent) SchemaVersion() int {
	return 1
}

func (e *MessageReceivedEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e *MessageReceivedEvent) Data() string {
	return e.data
}
