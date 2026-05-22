package slack

import (
	"bytes"
	"context"
	"encoding/base64"

	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func (e messageEnricher) enrichFiles(ctx context.Context, data *MessageReceivedData, event *slackevents.MessageEvent) {
	if event.Message == nil || len(event.Message.Files) == 0 {
		return
	}

	files := make([]FileAttachment, 0, len(event.Message.Files))
	images := make([]ImageAttachment, 0, len(event.Message.Files))
	for _, file := range event.Message.Files {
		switch {
		case isTextualSlackFile(file.Mimetype):
			attachment := e.enrichFile(ctx, file)
			if attachment.ID != "" {
				files = append(files, attachment)
			}
		case isSlackImageFile(file.Mimetype):
			image := e.enrichImage(ctx, file)
			if image.ID != "" {
				images = append(images, image)
			}
		default:
			files = append(files, unsupportedFileAttachment(file))
		}
	}
	data.Files = files
	data.Images = images
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

func (e messageEnricher) enrichImage(ctx context.Context, file slackgo.File) ImageAttachment {
	attachment := ImageAttachment{
		ID:          file.ID,
		Name:        file.Name,
		ContentType: file.Mimetype,
	}

	info, _, _, err := e.api.GetFileInfoContext(ctx, file.ID, 0, 0)
	if err != nil {
		e.log(ctx).WarnContext(ctx, "failed to get image info", "err", err, "file", file.ID)
		attachment.Status = ImageAttachmentStatusUnavailable
		attachment.Reason = err.Error()
		return attachment
	}
	if info == nil {
		attachment.Status = ImageAttachmentStatusUnavailable
		attachment.Reason = "missing file info"
		return attachment
	}
	if info.Name != "" {
		attachment.Name = info.Name
	}
	if info.Mimetype != "" {
		attachment.ContentType = info.Mimetype
	}
	if info.URLPrivateDownload == "" {
		attachment.Status = ImageAttachmentStatusUnavailable
		attachment.Reason = "missing download URL"
		return attachment
	}

	var buf bytes.Buffer
	if err := e.api.GetFileContext(ctx, info.URLPrivateDownload, &buf); err != nil {
		e.log(ctx).WarnContext(ctx, "failed to download image", "err", err, "file", file.ID)
		attachment.Status = ImageAttachmentStatusUnavailable
		attachment.Reason = err.Error()
		return attachment
	}
	attachment.Base64Data = base64.StdEncoding.EncodeToString(buf.Bytes())
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
