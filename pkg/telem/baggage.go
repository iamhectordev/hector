package telem

import (
	"context"

	"go.opentelemetry.io/otel/baggage"
)

// WithBaggage returns a context carrying fields through OpenTelemetry baggage.
func WithBaggage(ctx context.Context, fields ...Field) context.Context {
	members := make([]baggage.Member, 0, len(fields))
	for _, field := range fields {
		member, err := baggage.NewMember(field.key, fieldString(field))
		if err != nil {
			continue
		}
		members = append(members, member)
	}
	bag, err := baggage.New(members...)
	if err != nil {
		return ctx
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

func baggageFromContext(ctx context.Context) baggage.Baggage {
	return baggage.FromContext(ctx)
}
