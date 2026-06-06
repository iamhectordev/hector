package waffle

import "github.com/iamhectordev/hector/pkg/telem"

const (
	spanEventRecord = "waffle.event.record"
	spanReactionRun = "waffle.reaction.run"
)

func eventFields(event AnyEvent) []telem.Field {
	return []telem.Field{
		telem.String("waffle.event.id", event.ID()),
		telem.String("waffle.event.type", event.Type()),
		telem.Int("waffle.event.schema_version", event.SchemaVersion()),
	}
}

func handlerFields(handler registeredHandler) []telem.Field {
	return []telem.Field{
		telem.String("waffle.handler.name", handler.name),
		telem.String("waffle.event.type", handler.eventType),
	}
}

func reactionFields(reaction ReactionRecord) []telem.Field {
	return []telem.Field{
		telem.String("waffle.reaction.id", reaction.ID),
		telem.String("waffle.event.id", reaction.EventID),
		telem.String("waffle.handler.name", reaction.HandlerName),
	}
}
