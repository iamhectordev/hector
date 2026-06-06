package sqlite

import (
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
)

const (
	spanSessionLoad   = "session.load"
	spanSessionRecord = "session.record"
)

func sessionFields(sourceURI string) []telem.Field {
	fields := []telem.Field{}
	if parsed, err := session.ParseSourceURI(sourceURI); err == nil && parsed.Scheme != "" {
		fields = append(fields, telem.String("session.source_scheme", parsed.Scheme))
	}
	return fields
}
