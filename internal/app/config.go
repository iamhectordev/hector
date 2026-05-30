package app

import (
	kleelog "github.com/doron-cohen/klee/log"
	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/modules/github"
	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/pkg/llm"
)

// Config is the typed application config loaded by klee.
type Config struct {
	kleelog.Config `yaml:"log"`
	DB             dbsqlite.Config `yaml:"db"`
	LLM            llm.Config      `yaml:"llm"`
	WebSearch      search.Config   `yaml:"web_search"`
	GitHub         github.Config   `yaml:"github"`
	Slack          slack.Config    `yaml:"slack"`
}
