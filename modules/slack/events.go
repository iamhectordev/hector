package slack

import (
	"time"

	"github.com/iamhectordev/hector/pkg/waffle"
)

// MessageReceived is emitted when a Slack direct message is received.
var MessageReceived = mustDefine[MessageReceivedData]("slack.message_received", 1)

// MessageUpdated is emitted when a message is replaced (message_changed).
var MessageUpdated = mustDefine[MessageUpdatedData]("slack.message_updated", 1)

// ChannelType identifies the type of a Slack conversation.
type ChannelType string

const (
	ChannelTypeDM      ChannelType = "dm"
	ChannelTypeGroupDM ChannelType = "group_dm"
	ChannelTypeChannel ChannelType = "channel"
	ChannelTypePrivate ChannelType = "private_channel"
)

// Channel holds information about the conversation where the message was posted.
type Channel struct {
	ID          string
	Name        string
	Type        ChannelType
	MemberCount int
}

// Sender holds information about the user who sent the message.
type Sender struct {
	ID   string
	Name string
}

// Reaction is an emoji reaction attached to a Slack message.
type Reaction struct {
	Emoji string
	Count int
	You   bool
}

// Reactions is the best-effort reaction enrichment for a Slack message.
type Reactions struct {
	Items       []Reaction
	Unavailable *UnavailableReactions
}

// UnavailableReactions records why Slack reactions could not be enriched.
type UnavailableReactions struct {
	Reason string
}

// FileAttachment is a best-effort textual file attachment enrichment.
type FileAttachment struct {
	ID          string
	Name        string
	ContentType string
	Content     string
	Status      FileAttachmentStatus
	Reason      string
}

type FileAttachmentStatus string

const (
	FileAttachmentStatusUnavailable FileAttachmentStatus = "unavailable"
	FileAttachmentStatusUnsupported FileAttachmentStatus = "unsupported"
)

// ImageAttachment is a best-effort image attachment enrichment.
type ImageAttachment struct {
	ID          string
	Name        string
	ContentType string
	Base64Data  string
	Status      ImageAttachmentStatus
	Reason      string
}

type ImageAttachmentStatus string

const (
	ImageAttachmentStatusUnavailable ImageAttachmentStatus = "unavailable"
)

// MessageReceivedData is the payload for [MessageReceived].
type MessageReceivedData struct {
	Channel    Channel
	ThreadTS   string // Thread root's ts when replying in a thread; set to message's own ts for top-level messages.
	TS         string // Raw Slack timestamp from the event or forwarded-attachment Ts.
	Sender     Sender
	Text       string
	Reactions  Reactions
	Files      []FileAttachment
	Images     []ImageAttachment
	Forwards   []MessageReceivedData
	SentAt     time.Time
	ReceivedAt time.Time
}

// MessageUpdatedData is the payload for [MessageUpdated].
type MessageUpdatedData struct {
	Channel    Channel
	ThreadTS   string
	TS         string
	Sender     Sender
	Text       string
	Reactions  Reactions
	Files      []FileAttachment
	Images     []ImageAttachment
	Forwards   []MessageReceivedData
	SentAt     time.Time
	ReceivedAt time.Time
	UpdatedAt  time.Time
}

func mustDefine[T any](eventType string, schemaVersion int) waffle.Definition[T] {
	def, err := waffle.Define[T](eventType, schemaVersion)
	if err != nil {
		panic(err)
	}
	return def
}
