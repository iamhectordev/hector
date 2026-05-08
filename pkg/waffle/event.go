package waffle

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// Event is a producer-owned fact wrapped in Waffle metadata.
type Event[T any] interface {
	AnyEvent

	// Data returns the producer-owned payload.
	Data() T
}

// AnyEvent is the untyped view Waffle needs to store and route events.
type AnyEvent interface {
	// ID returns the unique identifier of the event.
	ID() string

	// Type returns the type of the event.
	Type() string

	// SchemaVersion returns the producer-owned schema version.
	SchemaVersion() int

	// OccurredAt returns when the event was created.
	OccurredAt() time.Time

	// Payload returns the producer-owned payload without its static type.
	Payload() any
}

// Definition describes a concrete event type and payload schema version.
type Definition[T any] struct {
	eventType     string
	schemaVersion int
}

// Define creates an event definition for a producer-owned payload type.
func Define[T any](eventType string, schemaVersion int) (Definition[T], error) {
	if eventType == "" {
		return Definition[T]{}, fmt.Errorf("waffle: event type cannot be empty")
	}
	if schemaVersion < 1 {
		return Definition[T]{}, fmt.Errorf("waffle: schema version must be positive")
	}

	return Definition[T]{
		eventType:     eventType,
		schemaVersion: schemaVersion,
	}, nil
}

// Type returns the stable event type used for routing.
func (d Definition[T]) Type() string {
	return d.eventType
}

// SchemaVersion returns the payload schema version for this definition.
func (d Definition[T]) SchemaVersion() int {
	return d.schemaVersion
}

// New wraps payload data in a Waffle event envelope.
func (d Definition[T]) New(data T) Event[T] {
	return event[T]{
		id:            "evt_" + ulid.Make().String(),
		eventType:     d.eventType,
		schemaVersion: d.schemaVersion,
		occurredAt:    time.Now().UTC(),
		data:          data,
	}
}

type event[T any] struct {
	id            string
	eventType     string
	schemaVersion int
	occurredAt    time.Time
	data          T
}

func (e event[T]) ID() string {
	return e.id
}

func (e event[T]) Type() string {
	return e.eventType
}

func (e event[T]) SchemaVersion() int {
	return e.schemaVersion
}

func (e event[T]) OccurredAt() time.Time {
	return e.occurredAt
}

func (e event[T]) Data() T {
	return e.data
}

func (e event[T]) Payload() any {
	return e.data
}
