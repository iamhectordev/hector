package slack

import (
	"time"

	"github.com/iamhectordev/hector/pkg/waffle"
)

var MessageReceived = mustDefine[MessageReceivedData]("slack.message_received", 1)

var MessageUpdated = mustDefine[MessageUpdatedData]("slack.message_updated", 1)

type ChannelType string

const (
	ChannelTypeDM      ChannelType = "dm"
	ChannelTypeGroupDM ChannelType = "group_dm"
	ChannelTypeChannel ChannelType = "channel"
	ChannelTypePrivate ChannelType = "private_channel"
)

type Channel struct {
	ID          string
	Name        string
	Type        ChannelType
	MemberCount int
}

type Sender struct {
	ID   string
	Name string
}

type Reaction struct {
	Emoji string
	Count int
	You   bool
}

type Reactions struct {
	Items       []Reaction
	Unavailable *UnavailableReactions
}

type UnavailableReactions struct {
	Reason string
}

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

type MessageReceivedData struct {
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
}

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
