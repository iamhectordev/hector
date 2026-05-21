package slack

import (
	"context"
	"errors"
	"io"
	"testing"

	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"
)

func TestMessageEnricher_EnrichFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		api  *fileAPI
		file slackgo.File
		want []FileAttachment
	}{
		{
			name: "downloads textual file",
			api: &fileAPI{
				info: &slackgo.File{
					ID:                 "F456",
					Name:               "config.md",
					Mimetype:           "text/markdown",
					URLPrivateDownload: "https://files.example/config.md",
				},
				content: "# Config\n",
			},
			file: slackgo.File{
				ID:       "F456",
				Name:     "config.md",
				Mimetype: "text/markdown",
			},
			want: []FileAttachment{{
				ID:          "F456",
				Name:        "config.md",
				ContentType: "text/markdown",
				Content:     "# Config\n",
			}},
		},
		{
			name: "marks binary file unsupported without metadata fetch",
			api:  &fileAPI{},
			file: slackgo.File{
				ID:       "F789",
				Name:     "photo.png",
				Mimetype: "image/png",
			},
			want: []FileAttachment{{
				ID:          "F789",
				Name:        "photo.png",
				ContentType: "image/png",
				Status:      FileAttachmentStatusUnsupported,
				Reason:      "non-textual file",
			}},
		},
		{
			name: "marks textual download failure unavailable",
			api: &fileAPI{
				info: &slackgo.File{
					ID:                 "F456",
					Name:               "config.md",
					Mimetype:           "text/markdown",
					URLPrivateDownload: "https://files.example/config.md",
				},
				downloadErr: errors.New("network error"),
			},
			file: slackgo.File{
				ID:       "F456",
				Name:     "config.md",
				Mimetype: "text/markdown",
			},
			want: []FileAttachment{{
				ID:          "F456",
				Name:        "config.md",
				ContentType: "text/markdown",
				Status:      FileAttachmentStatusUnavailable,
				Reason:      "network error",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var data MessageReceivedData
			messageEnricher{api: tt.api}.enrichFiles(t.Context(), &data, &slackevents.MessageEvent{
				Message: &slackgo.Msg{Files: []slackgo.File{tt.file}},
			})

			require.Equal(t, tt.want, data.Files)
			if tt.file.Mimetype == "image/png" {
				require.Zero(t, tt.api.infoCalls)
				require.Zero(t, tt.api.downloadCalls)
			}
		})
	}
}

func TestIsTextualSlackFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mime string
		want bool
	}{
		{mime: "text/plain", want: true},
		{mime: "text/markdown", want: true},
		{mime: "application/json", want: true},
		{mime: "application/x-yaml", want: true},
		{mime: "image/png", want: false},
		{mime: "application/pdf", want: false},
		{mime: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, isTextualSlackFile(tt.mime))
		})
	}
}

type fileAPI struct {
	info          *slackgo.File
	infoErr       error
	content       string
	downloadErr   error
	infoCalls     int
	downloadCalls int
}

func (a *fileAPI) GetUserInfoContext(context.Context, string) (*slackgo.User, error) {
	return &slackgo.User{}, nil
}

func (a *fileAPI) GetConversationInfoContext(context.Context, *slackgo.GetConversationInfoInput) (*slackgo.Channel, error) {
	return &slackgo.Channel{}, nil
}

func (a *fileAPI) GetReactionsContext(context.Context, slackgo.ItemRef, slackgo.GetReactionsParameters) (slackgo.ReactedItem, error) {
	return slackgo.ReactedItem{}, nil
}

func (a *fileAPI) GetFileInfoContext(context.Context, string, int, int) (*slackgo.File, []slackgo.Comment, *slackgo.Paging, error) {
	a.infoCalls++
	if a.infoErr != nil {
		return nil, nil, nil, a.infoErr
	}
	return a.info, nil, nil, nil
}

func (a *fileAPI) GetFileContext(_ context.Context, _ string, writer io.Writer) error {
	a.downloadCalls++
	if a.downloadErr != nil {
		return a.downloadErr
	}
	_, err := io.WriteString(writer, a.content)
	return err
}
