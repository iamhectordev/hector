package slack

import (
	"context"
	"io"
	"log/slog"

	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/sourcegraph/conc/pool"
)

type slackAPI interface {
	GetUserInfoContext(ctx context.Context, user string) (*slackgo.User, error)
	GetConversationInfoContext(ctx context.Context, input *slackgo.GetConversationInfoInput) (*slackgo.Channel, error)
	GetReactionsContext(ctx context.Context, item slackgo.ItemRef, params slackgo.GetReactionsParameters) (slackgo.ReactedItem, error)
	GetFileInfoContext(ctx context.Context, fileID string, count, page int) (*slackgo.File, []slackgo.Comment, *slackgo.Paging, error)
	GetFileContext(ctx context.Context, downloadURL string, writer io.Writer) error
}

type messageEnricher struct {
	api       slackAPI
	botUserID string
}

func (e messageEnricher) Enrich(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
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

func (e messageEnricher) enrichForward(ctx context.Context, data *MessageReceivedData) {
	p := pool.New()
	p.Go(func() { e.enrichSender(ctx, data) })
	p.Go(func() { e.enrichChannel(ctx, data) })
	p.Go(func() { e.enrichReactions(ctx, data) })
	p.Wait()
}

func (e messageEnricher) enrichSender(ctx context.Context, data *MessageReceivedData) {
	if data.Sender.ID == "" {
		return
	}
	user, err := e.api.GetUserInfoContext(ctx, data.Sender.ID)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get user info", "err", err, "user", data.Sender.ID)
		return
	}
	name := user.Profile.DisplayName
	if name == "" {
		name = user.Profile.RealName
	}
	data.Sender.Name = name
}

func (e messageEnricher) enrichChannel(ctx context.Context, data *MessageReceivedData) {
	if data.Channel.ID == "" {
		return
	}
	channel, err := e.api.GetConversationInfoContext(ctx, &slackgo.GetConversationInfoInput{
		ChannelID:         data.Channel.ID,
		IncludeLocale:     false,
		IncludeNumMembers: true,
	})
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get conversation info", "err", err, "channel", data.Channel.ID)
		return
	}
	data.Channel.Name = channel.Name
	data.Channel.MemberCount = channel.NumMembers
}

func (e messageEnricher) enrichReactions(ctx context.Context, data *MessageReceivedData) {
	if data.Channel.ID == "" || data.TS == "" {
		return
	}
	item, err := e.api.GetReactionsContext(
		ctx,
		slackgo.NewRefToMessage(data.Channel.ID, data.TS),
		slackgo.GetReactionsParameters{Full: true},
	)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get reactions", "err", err, "channel", data.Channel.ID, "ts", data.TS)
		data.Reactions.Unavailable = &UnavailableReactions{Reason: err.Error()}
		return
	}
	data.Reactions = reactionsFromSlack(item, e.botUserID)
}

func (e messageEnricher) log(context.Context) *slog.Logger {
	return slog.Default().With("component", "module", "module", "slack")
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
