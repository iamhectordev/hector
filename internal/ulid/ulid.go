package ulid

import "github.com/oklog/ulid/v2"

func New(prefix string) string {
	id := ulid.Make()
	return prefix + "_" + id.String()
}
