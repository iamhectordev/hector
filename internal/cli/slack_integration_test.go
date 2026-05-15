package cli_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/iamhectordev/hector/modules/agent"
	slackmodule "github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm/schema"
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

	registry, err := tools.NewRegistry(
		comms.NewReplyRouter(slackMod.NewReplyHandler()),
	)
	require.NoError(t, err)

	loop := agent.NewLoop(
		&scriptedCompleter{replies: []*schema.Message{
			withToolCall(schema.ToolCall{
				ID:        "c1",
				Name:      "reply",
				Arguments: json.RawMessage(`{"text":"hello back"}`),
			}),
			withStop(""),
		}},
		agent.WithTools(registry),
	)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, loop),
		slackMod,
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

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
}

// scriptedCompleter returns replies in order, one per Complete call.
type scriptedCompleter struct {
	replies []*schema.Message
	i       int
}

func (c *scriptedCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	if c.i >= len(c.replies) {
		return nil, nil
	}
	r := c.replies[c.i]
	c.i++
	return r, nil
}

func withStop(content string) *schema.Message {
	m := schema.AssistantMessage(content)
	m.FinishReason = schema.FinishReasonStop
	return m
}

func withToolCall(calls ...schema.ToolCall) *schema.Message {
	return &schema.Message{
		Role:         schema.RoleAssistant,
		FinishReason: schema.FinishReasonToolCalls,
		ToolCalls:    calls,
	}
}
