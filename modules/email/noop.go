package email

import "context"

// NoopMailbox is a mailbox implementation for slices that do not receive real email yet.
type NoopMailbox struct{}

func NewNoopMailbox() *NoopMailbox {
	return &NoopMailbox{}
}

func (m *NoopMailbox) FetchUnread(context.Context, int) ([]Email, error) {
	return nil, nil
}

func (m *NoopMailbox) MarkRead(context.Context, []MessageID) error {
	return nil
}

func (m *NoopMailbox) Close(context.Context) error {
	return nil
}
