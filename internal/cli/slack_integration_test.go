package cli_test

import (
	"context"
	"testing"
	"time"

	"github.com/iamhectordev/hector/modules/agent"
	slackmodule "github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/comms"
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
		agent.NewModule(bus, loop, agent.WithBaseSystem(agent.SystemPrompt)),
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

	require.NotEmpty(t, completer.Requests)
	sys := completer.Requests[0].System
	require.Contains(t, sys, `<conversation platform="slack" channel_type="dm" channel_id="D123" thread_ts="1610241741.000200">`)
	require.Contains(t, sys, `<participants>
    <participant id="U222"></participant>
  </participants>`)
}
