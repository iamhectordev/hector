package slack

import (
	"context"
	"io"

	"github.com/iamhectordev/hector/pkg/telem"
	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/sourcegraph/conc/pool"
)

type SlackAPI interface {
	GetUserInfoContext(ctx context.Context, user string) (*slackgo.User, error)
	GetConversationInfoContext(ctx context.Context, input *slackgo.GetConversationInfoInput) (*slackgo.Channel, error)
	GetReactionsContext(ctx context.Context, item slackgo.ItemRef, params slackgo.GetReactionsParameters) (slackgo.ReactedItem, error)
	GetFileInfoContext(ctx context.Context, fileID string, count, page int) (*slackgo.File, []slackgo.Comment, *slackgo.Paging, error)
	GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error
}

type MessageEnricher struct {
	api       SlackAPI
	botUserID string
}

func NewMessageEnricher(api SlackAPI, botUserID string) MessageEnricher {
	return MessageEnricher{api: api, botUserID: botUserID}
}

func (e MessageEnricher) Enrich(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
	p := pool.New()
	p.Go(func() { e.enrichSender(ctx, data) })
	p.Go(func() { e.enrichChannel(ctx, data) })
	p.Go(func() { e.enrichReactions(ctx, data) })
	p.Go(func() { e.enrichFiles(ctx, data, event) })
	p.Wait()

	for i := range data.Forwards {
		e.enrichForward(ctx, &data.Forwards[i])
	}
}

func (e MessageEnricher) enrichForward(ctx context.Context, data *MessageReceivedData) {
	p := pool.New()
	p.Go(func() { e.enrichSender(ctx, data) })
	p.Go(func() { e.enrichChannel(ctx, data) })
	p.Go(func() { e.enrichReactions(ctx, data) })
	p.Wait()
}

func (e MessageEnricher) enrichSender(ctx context.Context, data *MessageReceivedData) {
	if data.Sender.ID == "" {
		return
	}
	user, err := e.api.GetUserInfoContext(ctx, data.Sender.ID)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get user info",
			telem.Any("err", err),
			telem.String("user", data.Sender.ID),
		)
		return
	}
	name := user.Profile.DisplayName
	if name == "" {
		name = user.Profile.RealName
	}
	data.Sender.Name = name
}

func (e MessageEnricher) enrichChannel(ctx context.Context, data *MessageReceivedData) {
	if data.Channel.ID == "" {
		return
	}
	channel, err := e.api.GetConversationInfoContext(ctx, &slackgo.GetConversationInfoInput{
		ChannelID:         data.Channel.ID,
		IncludeLocale:     false,
		IncludeNumMembers: true,
	})
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get conversation info",
			telem.Any("err", err),
			telem.String("channel", data.Channel.ID),
		)
		return
	}
	data.Channel.Name = channel.Name
	data.Channel.MemberCount = channel.NumMembers
}

func (e MessageEnricher) enrichReactions(ctx context.Context, data *MessageReceivedData) {
	if data.Channel.ID == "" || data.TS == "" {
		return
	}
	item, err := e.api.GetReactionsContext(
		ctx,
		slackgo.NewRefToMessage(data.Channel.ID, data.TS),
		slackgo.GetReactionsParameters{Full: true},
	)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get reactions",
			telem.Any("err", err),
			telem.String("channel", data.Channel.ID),
			telem.String("ts", data.TS),
		)
		data.Reactions.Unavailable = &UnavailableReactions{Reason: err.Error()}
		return
	}
	data.Reactions = reactionsFromSlack(item, e.botUserID)
}

func (e MessageEnricher) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", "slack"),
	)
}

func reactionsFromSlack(item slackgo.ReactedItem, botUserID string) Reactions {
	reactions := Reactions{Items: make([]Reaction, 0, len(item.Reactions))}
	for _, reaction := range item.Reactions {
		reactions.Items = append(reactions.Items, Reaction{
			Emoji: ":" + reaction.Name + ":",
			Count: reaction.Count,
			You:   reactionIncludesUser(reaction.Users, botUserID),
		})
	}
	return reactions
}

func reactionIncludesUser(users []string, userID string) bool {
	for _, user := range users {
		if user == userID {
			return true
		}
	}
	return false
}
