package emailtest_test

import (
	"testing"

	"github.com/iamhectordev/hector/modules/email"
	"github.com/iamhectordev/hector/modules/email/emailtest"
	"github.com/stretchr/testify/require"
)

func TestMailboxTracksUnreadAndReadMessages(t *testing.T) {
	t.Parallel()

	mailbox := emailtest.NewMailbox(
		email.Email{ID: email.MessageID("msg-1"), Subject: "one"},
		email.Email{ID: email.MessageID("msg-2"), Subject: "two"},
	)

	require.NoError(t, mailbox.MarkRead(t.Context(), []email.MessageID{email.MessageID("msg-1")}))

	messages, err := mailbox.FetchUnread(t.Context(), 10)
	require.NoError(t, err)
	require.Equal(t, []email.Email{{ID: email.MessageID("msg-2"), Subject: "two"}}, messages)
	require.True(t, mailbox.IsRead(email.MessageID("msg-1")))
	require.False(t, mailbox.IsRead(email.MessageID("msg-2")))
}

func TestMailboxLimitsUnreadMessages(t *testing.T) {
	t.Parallel()

	mailbox := emailtest.NewMailbox(
		email.Email{ID: email.MessageID("msg-1")},
		email.Email{ID: email.MessageID("msg-2")},
	)

	messages, err := mailbox.FetchUnread(t.Context(), 1)

	require.NoError(t, err)
	require.Equal(t, []email.Email{{ID: email.MessageID("msg-1")}}, messages)
}
