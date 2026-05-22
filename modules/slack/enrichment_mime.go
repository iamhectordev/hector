package slack

import "strings"

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

func isSlackImageFile(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	return strings.HasPrefix(mimeType, "image/")
}
