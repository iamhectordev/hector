package cli_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	slackmock "github.com/iamhectordev/hector/internal/slack/mock"
	"github.com/iamhectordev/hector/modules/agent"
	slackmodule "github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSlack_DMMessage_RepliesInThread(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	srv := slackmock.New(t)

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	slackMod, err := slackmodule.NewModule(bus, testSlackConfig(t, srv.BaseURL()+"/api/"))
	require.NoError(t, err)

	replyRouter, err := comms.NewReplyRouter(slackMod.NewReplyHandler())
	require.NoError(t, err)
	registry, err := tools.NewRegistry(replyRouter)
	require.NoError(t, err)

	completer := llmtest.NewCompleter(t,
		llmtest.ToolCalls(llmtest.Call("c1", "reply", `{"text":"hello back"}`)),
		llmtest.Stop(""),
	)
	loop := agent.NewLoop(
		completer,
		agent.WithTools(registry),
	)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, loop,
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithSessionStore(noopSessionStore{}),
		),
		slackMod,
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id": "U222",
			"profile": map[string]any{
				"display_name": "Test User",
			},
		},
	})
	reactionsGet := srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{
			"type": "message",
			"user": "U222",
			"text": "hi",
			"ts":   "1610241741.000200",
			"reactions": []map[string]any{
				{"name": "eyes", "count": 2, "users": []string{"U333", "U444"}},
				{"name": "thumbsup", "count": 5, "users": []string{"U111", "U555"}},
			},
		},
	})
	postMessage := srv.Expect("chat.postMessage")

	err = srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "hi",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	})
	require.NoError(t, err)

	call := postMessage.Require(t, ctx)
	require.Equal(t, "D123", call.Get("channel"))
	require.Equal(t, "hello back", call.Get("text"))
	require.Equal(t, "1610241741.000200", call.Get("thread_ts"))
	reactionsCall := reactionsGet.Require(t, ctx)
	require.Equal(t, "D123", reactionsCall.Get("channel"))
	require.Equal(t, "1610241741.000200", reactionsCall.Get("timestamp"))
	require.Equal(t, "true", reactionsCall.Get("full"))

	require.NotEmpty(t, completer.Requests)
	req := completer.Requests[0]
	require.Contains(t, req.System, `<conversation platform="slack" channel_type="dm" channel_id="D123" thread_ts="1610241741.000200"></conversation>`)

	require.Len(t, req.Messages, 1)
	require.Equal(t, `<msg sender_id="U222" sender_name="Test User">
  <text>hi</text>
  <reactions>
    <r emoji=":eyes:" count="2"></r>
    <r emoji=":thumbsup:" count="5" you="true"></r>
  </reactions>
</msg>`, req.Messages[0].Content)
}

func TestSlack_DMMessage_TraceShape(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	srv := slackmock.New(t)
	db, err := dbsqlite.Open(ctx, dbsqlite.Config{Path: filepath.Join(t.TempDir(), "hector.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations()))

	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithStore(wafflesqlite.NewStore(db)),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)

	slackMod, err := slackmodule.NewModule(bus, testSlackConfig(t, srv.BaseURL()+"/api/"))
	require.NoError(t, err)

	replyRouter, err := comms.NewReplyRouter(slackMod.NewReplyHandler())
	require.NoError(t, err)
	registry, err := tools.NewRegistry(replyRouter)
	require.NoError(t, err)
	completer, err := llm.New(t.Context(), &llm.Config{DefaultBackend: llm.BackendEcho})
	require.NoError(t, err)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, agent.NewLoop(completer, agent.WithTools(registry)),
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithSessionStore(noopSessionStore{}),
		),
		slackMod,
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)
	go sv.Run(ctx)
	t.Cleanup(func() {
		cancel()
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{"type": "message", "user": "U222", "text": "hi", "ts": "1610241741.000200"},
	})
	postMessage := srv.Expect("chat.postMessage")

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "hi",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	}))
	call := postMessage.Require(t, ctx)
	require.Contains(t, call.Get("text"), "hi")

	require.Eventually(t, func() bool {
		return traceHasSpanNames(recorder.Ended(),
			"slack.message.receive",
			"waffle.event.record",
			"waffle.reaction.run",
			"agent.turn.run",
			"llm.complete",
			"tool.call",
			"tool.registry.run",
			"tool.reply.route",
			"slack.reply.send",
		)
	}, 2*time.Second, 20*time.Millisecond)

	spans := recorder.Ended()
	requireTraceTree(t, spans,
		"slack.message.receive",
		"waffle.event.record",
		"waffle.reaction.run",
		"agent.turn.run",
		"llm.complete",
		"tool.call",
		"tool.registry.run",
		"tool.reply.route",
		"slack.reply.send",
	)
	requireSpanAttr(t, spans, "llm.complete", "llm.provider", "echo")
	requireSpanAttr(t, spans, "waffle.reaction.run", "waffle.reaction.id", "")
	requireNoSpanAttrValue(t, spans, "hi")
}

func TestSlack_DMMessage_ReactionFailureStillReachesAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	srv := slackmock.New(t)

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	slackMod, err := slackmodule.NewModule(bus, testSlackConfig(t, srv.BaseURL()+"/api/"))
	require.NoError(t, err)

	completer := newCaptureCompleter()
	loop := agent.NewLoop(completer)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, loop,
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithSessionStore(noopSessionStore{}),
		),
		slackMod,
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":    false,
		"error": "network_error",
	})

	err = srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "hi",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	})
	require.NoError(t, err)

	var req schema.CompletionRequest
	select {
	case req = <-completer.requests:
	case <-ctx.Done():
		t.Fatal("timed out waiting for agent request")
	}
	require.Len(t, req.Messages, 1)
	require.Equal(t, `<msg sender_id="U222" sender_name="Test User">
  <text>hi</text>
  <reactions status="unavailable" reason="network_error"></reactions>
</msg>`, req.Messages[0].Content)
}

func TestSlack_DMMessage_TextFileAttachment_ReachesAgentInline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer xoxb-fake", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/markdown")
		_, err := w.Write([]byte("# Config\n\nport: 8080\n"))
		require.NoError(t, err)
	}))
	t.Cleanup(fileServer.Close)

	srv, completer := newSlackAgentCapture(t, ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{"type": "message", "ts": "1610241741.000200"},
	})
	filesInfo := srv.ExpectWithResponse("files.info", map[string]any{
		"ok": true,
		"file": map[string]any{
			"id":                   "F456",
			"name":                 "config.md",
			"mimetype":             "text/markdown",
			"url_private_download": fileServer.URL + "/config.md",
		},
	})

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "please inspect",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
		Message: &slackgo.Msg{
			Files: []slackgo.File{{
				ID:       "F456",
				Name:     "config.md",
				Mimetype: "text/markdown",
			}},
		},
	})
	require.NoError(t, err)

	filesInfoCall := filesInfo.Require(t, ctx)
	require.Equal(t, "F456", filesInfoCall.Get("file"))

	var req schema.CompletionRequest
	select {
	case req = <-completer.requests:
	case <-ctx.Done():
		t.Fatal("timed out waiting for agent request")
	}
	require.Len(t, req.Messages, 1)
	require.Equal(t, `<msg sender_id="U222" sender_name="Test User">
  <text>please inspect</text>
  <file id="F456" name="config.md" type="text/markdown"># Config

port: 8080
</file>
</msg>`, req.Messages[0].Content)
}

func TestSlack_DMMessage_ImageAttachment_ReachesAgentAsImagePart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	imageBytes := []byte{0x89, 'P', 'N', 'G'}
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer xoxb-fake", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		_, err := w.Write(imageBytes)
		require.NoError(t, err)
	}))
	t.Cleanup(fileServer.Close)

	srv, completer := newSlackAgentCapture(t, ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{"type": "message", "ts": "1610241741.000200"},
	})
	filesInfo := srv.ExpectWithResponse("files.info", map[string]any{
		"ok": true,
		"file": map[string]any{
			"id":                   "F789",
			"name":                 "screenshot.png",
			"mimetype":             "image/png",
			"url_private_download": fileServer.URL + "/screenshot.png",
		},
	})

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "please inspect",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
		Message: &slackgo.Msg{
			Files: []slackgo.File{{
				ID:       "F789",
				Name:     "screenshot.png",
				Mimetype: "image/png",
			}},
		},
	})
	require.NoError(t, err)

	filesInfoCall := filesInfo.Require(t, ctx)
	require.Equal(t, "F789", filesInfoCall.Get("file"))

	var req schema.CompletionRequest
	select {
	case req = <-completer.requests:
	case <-ctx.Done():
		t.Fatal("timed out waiting for agent request")
	}
	require.Len(t, req.Messages, 1)
	require.Equal(t, `<msg sender_id="U222" sender_name="Test User">
  <text>please inspect</text>
  <img id="F789" name="screenshot.png" type="image/png"></img>
</msg>`, req.Messages[0].Content)
	require.Equal(t, []schema.MessagePart{
		schema.TextPart(req.Messages[0].Content),
		schema.TextPart(`<image_data id="F789"/>`),
		schema.NewImagePart("F789", base64.StdEncoding.EncodeToString(imageBytes), "image/png"),
	}, req.Messages[0].Parts)
}

func TestSlack_DMMessage_ImageDownloadFailureStillReachesAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(fileServer.Close)

	srv, completer := newSlackAgentCapture(t, ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{"type": "message", "ts": "1610241741.000200"},
	})
	srv.ExpectWithResponse("files.info", map[string]any{
		"ok": true,
		"file": map[string]any{
			"id":                   "F789",
			"name":                 "screenshot.png",
			"mimetype":             "image/png",
			"url_private_download": fileServer.URL + "/screenshot.png",
		},
	})

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "please inspect",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
		Message: &slackgo.Msg{
			Files: []slackgo.File{{
				ID:       "F789",
				Name:     "screenshot.png",
				Mimetype: "image/png",
			}},
		},
	})
	require.NoError(t, err)

	var req schema.CompletionRequest
	select {
	case req = <-completer.requests:
	case <-ctx.Done():
		t.Fatal("timed out waiting for agent request")
	}
	require.Len(t, req.Messages, 1)
	require.Contains(t, req.Messages[0].Content, `<text>please inspect</text>`)
	require.Contains(t, req.Messages[0].Content, `<img id="F789" name="screenshot.png" type="image/png" status="unavailable" reason=`)
	require.Empty(t, req.Messages[0].Parts)
}

func TestSlack_DMMessage_TextFileDownloadFailureStillReachesAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(fileServer.Close)

	srv, completer := newSlackAgentCapture(t, ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{"type": "message", "ts": "1610241741.000200"},
	})
	srv.ExpectWithResponse("files.info", map[string]any{
		"ok": true,
		"file": map[string]any{
			"id":                   "F456",
			"name":                 "config.md",
			"mimetype":             "text/markdown",
			"url_private_download": fileServer.URL + "/config.md",
		},
	})

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "please inspect",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
		Message: &slackgo.Msg{
			Files: []slackgo.File{{
				ID:       "F456",
				Name:     "config.md",
				Mimetype: "text/markdown",
			}},
		},
	})
	require.NoError(t, err)

	var req schema.CompletionRequest
	select {
	case req = <-completer.requests:
	case <-ctx.Done():
		t.Fatal("timed out waiting for agent request")
	}
	require.Len(t, req.Messages, 1)
	require.Contains(t, req.Messages[0].Content, `<text>please inspect</text>`)
	require.Contains(t, req.Messages[0].Content, `<file id="F456" name="config.md" type="text/markdown" status="unavailable" reason=`)
}

func TestSlack_DMMessage_BinaryFileAttachment_IsMarkedUnsupported(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	srv, completer := newSlackAgentCapture(t, ctx)

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Test User"},
		},
	})
	srv.ExpectWithResponse("reactions.get", map[string]any{
		"ok":      true,
		"type":    "message",
		"channel": "D123",
		"message": map[string]any{"type": "message", "ts": "1610241741.000200"},
	})

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "please inspect",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
		Message: &slackgo.Msg{
			Files: []slackgo.File{{
				ID:       "F789",
				Name:     "document.pdf",
				Mimetype: "application/pdf",
			}},
		},
	})
	require.NoError(t, err)

	var req schema.CompletionRequest
	select {
	case req = <-completer.requests:
	case <-ctx.Done():
		t.Fatal("timed out waiting for agent request")
	}
	require.Len(t, req.Messages, 1)
	require.Equal(t, `<msg sender_id="U222" sender_name="Test User">
  <text>please inspect</text>
  <file id="F789" name="document.pdf" type="application/pdf" status="unsupported" reason="non-textual file"></file>
</msg>`, req.Messages[0].Content)
}

type captureCompleter struct {
	requests chan schema.CompletionRequest
}

func newSlackAgentCapture(t *testing.T, ctx context.Context) (*slackmock.Server, *captureCompleter) {
	t.Helper()

	srv := slackmock.New(t)

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	slackMod, err := slackmodule.NewModule(bus, testSlackConfig(t, srv.BaseURL()+"/api/"))
	require.NoError(t, err)

	completer := newCaptureCompleter()
	loop := agent.NewLoop(completer)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, loop,
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithSessionStore(noopSessionStore{}),
		),
		slackMod,
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)
	return srv, completer
}

func newCaptureCompleter() *captureCompleter {
	return &captureCompleter{requests: make(chan schema.CompletionRequest, 1)}
}

func (c *captureCompleter) Complete(_ context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	c.requests <- req
	reply := schema.AssistantMessage("")
	reply.FinishReason = schema.FinishReasonStop
	return reply, nil
}

func traceHasSpanNames(spans []sdktrace.ReadOnlySpan, names ...string) bool {
	seen := make(map[string]struct{}, len(spans))
	for _, span := range spans {
		seen[span.Name()] = struct{}{}
	}
	for _, name := range names {
		if _, ok := seen[name]; !ok {
			return false
		}
	}
	return true
}

func requireTraceTree(t *testing.T, spans []sdktrace.ReadOnlySpan, names ...string) {
	t.Helper()

	for _, name := range names {
		require.True(t, traceHasSpanNames(spans, name), "missing span %q", name)
	}

	root := requireSpan(t, spans, "slack.message.receive")
	eventRecord := requireChildSpan(t, spans, root, "waffle.event.record")
	reactionRun := requireChildSpan(t, spans, eventRecord, "waffle.reaction.run")
	agentTurn := requireChildSpan(t, spans, reactionRun, "agent.turn.run")
	requireChildSpan(t, spans, agentTurn, "llm.complete")
	toolCall := requireChildSpan(t, spans, agentTurn, "tool.call")
	registryRun := requireChildSpan(t, spans, toolCall, "tool.registry.run")
	replyRoute := requireChildSpan(t, spans, registryRun, "tool.reply.route")
	requireChildSpan(t, spans, replyRoute, "slack.reply.send")
}

func requireSpan(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range spans {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("missing span %q", name)
	return nil
}

func requireChildSpan(t *testing.T, spans []sdktrace.ReadOnlySpan, parent sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	parentSpanID := parent.SpanContext().SpanID().String()
	traceID := parent.SpanContext().TraceID().String()
	for _, span := range spans {
		if span.Name() == name && span.Parent().SpanID().String() == parentSpanID {
			require.Equal(t, traceID, span.SpanContext().TraceID().String(), "span %q should be in the same trace", name)
			return span
		}
	}
	t.Fatalf("missing child span %q under %q", name, parent.Name())
	return nil
}

func requireSpanAttr(t *testing.T, spans []sdktrace.ReadOnlySpan, spanName, key, want string) {
	t.Helper()
	for _, span := range spans {
		if span.Name() != spanName {
			continue
		}
		for _, attr := range span.Attributes() {
			if string(attr.Key) != key {
				continue
			}
			if want == "" {
				require.NotEmpty(t, attr.Value.AsString())
			} else {
				require.Equal(t, want, attr.Value.AsString())
			}
			return
		}
	}
	t.Fatalf("missing attr %q on span %q", key, spanName)
}

func requireNoSpanAttrValue(t *testing.T, spans []sdktrace.ReadOnlySpan, forbidden string) {
	t.Helper()
	for _, span := range spans {
		for _, attr := range span.Attributes() {
			require.NotEqual(t, forbidden, attr.Value.AsString(), "raw content leaked on span %q attr %q", span.Name(), attr.Key)
		}
	}
}

func testSlackConfig(t *testing.T, apiURL string) *slackmodule.Config {
	t.Helper()
	cfg := &slackmodule.Config{APIURL: apiURL}
	require.NoError(t, cfg.AppToken.UnmarshalText([]byte("xapp-fake")))
	require.NoError(t, cfg.BotToken.UnmarshalText([]byte("xoxb-fake")))
	return cfg
}
