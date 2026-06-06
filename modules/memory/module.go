package memory

import (
	"context"
	"time"

	"github.com/iamhectordev/hector/internal/ulid"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/llm/structured"
	pkgmem "github.com/iamhectordev/hector/pkg/memory"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
)

const extractionSystem = "Extract self-contained facts worth remembering from this conversation. " +
	"Each fact should be concise, specific, and independently meaningful."

type memoryStore interface {
	Put(ctx context.Context, obj pkgmem.Object) error
}

type sessionStore interface {
	Messages(ctx context.Context, sourceURI string) ([]*schema.Message, error)
}

type extractionResult struct {
	Objects []extractedObject `json:"objects"`
}

type extractedObject struct {
	Content string `json:"content" jsonschema:"a self-contained fact worth remembering"`
}

// Module subscribes to agent.TurnEnd and extracts memory objects from each turn.
type Module struct {
	bus       *waffle.EventBus
	store     memoryStore
	sessions  sessionStore
	extractor *structured.Extractor[extractionResult]
}

// NewModule creates a Module. completer is used to extract facts from each turn.
func NewModule(bus *waffle.EventBus, store memoryStore, sessions sessionStore, completer llm.Completer) *Module {
	extractor, err := structured.NewExtractor[extractionResult](completer, extractionSystem)
	if err != nil {
		panic("memory: failed to build extractor: " + err.Error())
	}
	return &Module{
		bus:       bus,
		store:     store,
		sessions:  sessions,
		extractor: extractor,
	}
}

func (m *Module) Name() string { return "memory" }

func (m *Module) Init(ctx context.Context) error {
	return waffle.On(m.bus, agent.TurnEnd).Handle("memory.ingest", m.onTurnEnd)
}

func (m *Module) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m *Module) Stop(context.Context) error { return nil }

func (m *Module) onTurnEnd(ctx context.Context, e waffle.Event[agent.TurnEndData]) error {
	data := e.Data()

	messages, err := m.sessions.Messages(ctx, data.SourceURI)
	if err != nil {
		m.log(ctx).WarnContext(ctx, "memory: failed to fetch session", telem.Any("err", err))
		return nil
	}

	turn := messages
	if data.TurnOffset > 0 && data.TurnOffset < len(messages) {
		turn = messages[data.TurnOffset:]
	}
	if len(turn) == 0 {
		return nil
	}

	result, err := m.extractor.Extract(ctx, turn)
	if err != nil {
		m.log(ctx).WarnContext(ctx, "memory: extraction failed", telem.Any("err", err))
		return nil
	}

	now := time.Now().UTC()
	for _, obj := range result.Objects {
		if err := m.store.Put(ctx, pkgmem.Object{
			ID:        ulid.New("mem"),
			Content:   obj.Content,
			SessionID: data.SessionID,
			CreatedAt: now,
		}); err != nil {
			m.log(ctx).WarnContext(ctx, "memory: failed to store object", telem.Any("err", err))
		}
	}
	return nil
}

func (m *Module) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", "memory"),
	)
}
