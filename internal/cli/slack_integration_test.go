package cli_test

import (
	"context"
	"testing"
	"time"

	"github.com/iamhectordev/hector/modules/agent"
	slackmodule "github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/slackmock"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"
)

func TestSlack_DMMessage_RepliesInThread(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	srv := slackmock.New(t)

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	slackMod, err := slackmodule.NewModule(bus, slackmodule.Config{
		AppToken: "xapp-fake",
		BotToken: "xoxb-fake",
		APIURL:   srv.BaseURL() + "/api/",
	})
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

func TestSlack_DMMessage_ReactionFailureStillReachesAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	srv := slackmock.New(t)

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	slackMod, err := slackmodule.NewModule(bus, slackmodule.Config{
		AppToken: "xapp-fake",
		BotToken: "xoxb-fake",
		APIURL:   srv.BaseURL() + "/api/",
	})
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

type captureCompleter struct {
	requests chan schema.CompletionRequest
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
