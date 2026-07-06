// Package emailtest provides test doubles for the email module.
package emailtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/iamhectordev/hector/modules/email"
)

// Mailbox is an in-memory IMAP mailbox test double.
type Mailbox struct {
	mu       sync.Mutex
	messages []email.Email
	read     map[email.MessageID]bool
	closed   bool
}

func NewMailbox(messages ...email.Email) *Mailbox {
	copied := append([]email.Email(nil), messages...)
	return &Mailbox{
		messages: copied,
		read:     map[email.MessageID]bool{},
	}
}

func (m *Mailbox) Add(message email.Email) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, message)
}

func (m *Mailbox) FetchUnread(_ context.Context, limit int) ([]email.Email, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("emailtest: limit must be positive")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	messages := make([]email.Email, 0, min(limit, len(m.messages)))
	for _, message := range m.messages {
		if m.read[message.ID] {
			continue
		}
		messages = append(messages, message)
		if len(messages) == limit {
			break
		}
	}
	return messages, nil
}

func (m *Mailbox) MarkRead(_ context.Context, ids []email.MessageID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		m.read[id] = true
	}
	return nil
}

func (m *Mailbox) IsRead(id email.MessageID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.read[id]
}

func (m *Mailbox) Close(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

func (m *Mailbox) Closed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.closed
}
