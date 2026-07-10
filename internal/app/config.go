package app

import (
	kleelog "github.com/doron-cohen/klee/log"
	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/internal/email"
	gh "github.com/iamhectordev/hector/internal/github"
	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/modules/agent"
	slack "github.com/iamhectordev/hector/integrations/slack"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/memory"
)

// Config is the typed application config loaded by klee.
type Config struct {
	kleelog.Config `yaml:"log"`
	DB             dbsqlite.Config `yaml:"db"`
	LLM            llm.Config      `yaml:"llm"`
	Agent          agent.Config    `yaml:"agent"`
	Memory         memory.Config   `yaml:"memory"`
	Tracing        tracing.Config  `yaml:"tracing"`
	WebSearch      search.Config   `yaml:"web_search"`
	GitHub         gh.Config       `yaml:"github"`
	Email          email.Config    `yaml:"email"`
	Slack          slack.Config    `yaml:"slack"`
}
