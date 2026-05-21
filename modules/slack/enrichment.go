package slack

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"

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
	p.Go(func() { e.enrichSender(ctx, data, event) })
	p.Go(func() { e.enrichChannel(ctx, data, event) })
	p.Go(func() { e.enrichReactions(ctx, data, event) })
	p.Go(func() { e.enrichFiles(ctx, data, event) })
	p.Wait()
}

func (e messageEnricher) enrichSender(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
	user, err := e.api.GetUserInfoContext(ctx, event.User)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get user info", "err", err, "user", event.User)
		return
	}
	name := user.Profile.DisplayName
	if name == "" {
		name = user.Profile.RealName
	}
	data.Sender.Name = name
}

func (e messageEnricher) enrichChannel(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
	channel, err := e.api.GetConversationInfoContext(ctx, &slackgo.GetConversationInfoInput{
		ChannelID:         event.Channel,
		IncludeLocale:     false,
		IncludeNumMembers: true,
	})
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get conversation info", "err", err, "channel", event.Channel)
		return
	}
	data.Channel.Name = channel.Name
	data.Channel.MemberCount = channel.NumMembers
}

func (e messageEnricher) enrichReactions(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
	item, err := e.api.GetReactionsContext(
		ctx,
		slackgo.NewRefToMessage(event.Channel, event.TimeStamp),
		slackgo.GetReactionsParameters{Full: true},
	)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get reactions", "err", err, "channel", event.Channel, "ts", event.TimeStamp)
		data.Reactions.Unavailable = &UnavailableReactions{Reason: err.Error()}
		return
	}
	data.Reactions = reactionsFromSlack(item, e.botUserID)
}

func (e messageEnricher) enrichFiles(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
	if event.Message == nil || len(event.Message.Files) == 0 {
		return
	}

	files := make([]FileAttachment, 0, len(event.Message.Files))
	for _, file := range event.Message.Files {
		if !isTextualSlackFile(file.Mimetype) {
			files = append(files, unsupportedFileAttachment(file))
			continue
		}
		attachment := e.enrichFile(ctx, file)
		if attachment.ID != "" {
			files = append(files, attachment)
		}
	}
	data.Files = files
}

func (e messageEnricher) enrichFile(ctx context.Context, file slackgo.File) FileAttachment {
	attachment := FileAttachment{
		ID:          file.ID,
		Name:        file.Name,
		ContentType: file.Mimetype,
	}

	info, _, _, err := e.api.GetFileInfoContext(ctx, file.ID, 0, 0)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get file info", "err", err, "file", file.ID)
		attachment.Status = FileAttachmentStatusUnavailable
		attachment.Reason = err.Error()
		return attachment
	}
	if info.Name != "" {
		attachment.Name = info.Name
	}
	if info.Mimetype != "" {
		attachment.ContentType = info.Mimetype
	}
	if !isTextualSlackFile(attachment.ContentType) {
		attachment.Status = FileAttachmentStatusUnsupported
		attachment.Reason = "non-textual file"
		return attachment
	}
	if info.URLPrivateDownload == "" {
		attachment.Status = FileAttachmentStatusUnavailable
		attachment.Reason = "missing download URL"
		return attachment
	}

	var buf bytes.Buffer
	if err := e.api.GetFileContext(ctx, info.URLPrivateDownload, &buf); err != nil {
		e.log(ctx).WarnContext(ctx, "failed to download file", "err", err, "file", file.ID)
		attachment.Status = FileAttachmentStatusUnavailable
		attachment.Reason = err.Error()
		return attachment
	}
	attachment.Content = buf.String()
	return attachment
}

func unsupportedFileAttachment(file slackgo.File) FileAttachment {
	return FileAttachment{
		ID:          file.ID,
		Name:        file.Name,
		ContentType: file.Mimetype,
		Status:      FileAttachmentStatusUnsupported,
		Reason:      "non-textual file",
	}
}

func (e messageEnricher) log(context.Context) *slog.Logger {
	return slog.Default().With("component", "module", "module", "slack")
}

func isTextualSlackFile(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "" {
		return false
	}
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	switch mimeType {
	case "application/json",
		"application/javascript",
		"application/typescript",
		"application/xml",
		"application/x-httpd-php",
		"application/x-javascript",
		"application/x-python-code",
		"application/x-ruby",
		"application/x-sh",
		"application/x-yaml",
		"application/yaml":
		return true
	default:
		return false
	}
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
