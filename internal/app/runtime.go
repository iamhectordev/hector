package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/github"
	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/modules/tools/web"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/safehttp"
	sessionsqlite "github.com/iamhectordev/hector/pkg/session/sqlite"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
)

// Runtime wires and runs the long-running Hector application.
type Runtime struct {
	cfg    Config
	logger *slog.Logger

	db  *sql.DB
	bus *waffle.EventBus
	sv  *supervisor.Supervisor
}

// NewRuntime builds a Runtime from typed application config.
func NewRuntime(cfg Config, opts ...Option) (*Runtime, error) {
	r := &Runtime{
		cfg:    cfg,
		logger: slog.Default().With("component", "runtime"),
	}
	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("app: option is required")
		}
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Start initializes the application, runs it until shutdown, and closes owned resources.
func (r *Runtime) Start(ctx context.Context) error {
	r.logger.InfoContext(ctx, "starting app runtime")
	defer r.close(ctx)

	if err := r.init(ctx); err != nil {
		return err
	}

	rep := r.sv.Run(ctx)
	r.logger.InfoContext(ctx, "app runtime finished",
		"reason", rep.Reason,
		"trigger_module", rep.TriggerModule,
		"signal", rep.Signal,
	)
	return rep.Err()
}

func (r *Runtime) init(ctx context.Context) error {
	if err := r.initDatabase(ctx); err != nil {
		return err
	}
	if err := r.initBus(); err != nil {
		return err
	}

	completer, err := llm.New(r.cfg.LLM)
	if err != nil {
		return err
	}
	webSearch, err := newWebSearchTool(r.cfg.WebSearch)
	if err != nil {
		return err
	}
	slackModule, err := slack.NewModule(r.bus, r.cfg.Slack)
	if err != nil {
		return err
	}
	toolRegistry, toolsModule, err := r.initTools(slackModule, webSearch)
	if err != nil {
		return err
	}
	modules, err := r.initModules(completer, toolRegistry, toolsModule, slackModule)
	if err != nil {
		return err
	}
	return r.initSupervisor(modules)
}

func (r *Runtime) initDatabase(ctx context.Context) error {
	db, err := dbsqlite.Open(ctx, r.cfg.DB)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to open sqlite database", "err", err)
		return err
	}
	r.db = db

	if err := dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations(), sessionsqlite.Migrations()); err != nil {
		r.logger.ErrorContext(ctx, "failed to migrate sqlite database", "err", err)
		return fmt.Errorf("sqlite migrations: %w", err)
	}
	return nil
}

func (r *Runtime) initBus() error {
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithLogger(r.logger),
		waffle.WithStore(wafflesqlite.NewStore(r.db)),
		waffle.WithPersistentReactions(),
	)
	if err != nil {
		r.logger.Error("failed to create event bus", "err", err)
		return err
	}
	r.bus = bus
	return nil
}

func (r *Runtime) initTools(slackModule *slack.Module, webSearch tools.Tool) (*tools.Registry, *tools.Module, error) {
	replyRouter, err := comms.NewReplyRouter(slackModule.NewReplyHandler())
	if err != nil {
		return nil, nil, err
	}
	timeNow, err := tools.NewTimeNow()
	if err != nil {
		return nil, nil, err
	}
	httpClient, err := safehttp.Client()
	if err != nil {
		return nil, nil, err
	}
	webFetch, err := web.NewFetch(httpClient)
	if err != nil {
		return nil, nil, err
	}
	toolRegistry, err := tools.NewRegistry(replyRouter, timeNow, webFetch, webSearch)
	if err != nil {
		return nil, nil, err
	}
	toolsModule, err := tools.NewModule(r.bus, toolRegistry)
	if err != nil {
		return nil, nil, err
	}
	return toolRegistry, toolsModule, nil
}

func (r *Runtime) initModules(
	completer llm.Completer,
	toolRegistry *tools.Registry,
	toolsModule *tools.Module,
	slackModule *slack.Module,
) ([]supervisor.Module, error) {
	modules := []supervisor.Module{}
	if r.cfg.GitHub.Configured() {
		githubModule, err := github.NewModule(
			r.cfg.GitHub,
			github.WithLogger(r.logger.With("component", "module", "module", "github")),
			github.WithToolRegistrar(toolRegistry),
		)
		if err != nil {
			return nil, err
		}
		modules = append(modules, githubModule)
	}

	loop := agent.NewLoop(completer,
		agent.WithTools(toolRegistry),
		agent.WithLogger(r.logger.With("component", "loop")),
	)
	sessionStore := sessionsqlite.NewStore(r.db)
	modules = append(modules,
		agent.NewModule(r.bus, loop,
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithSessionStore(sessionStore),
		),
		toolsModule,
		slackModule,
	)
	return modules, nil
}

func (r *Runtime) initSupervisor(modules []supervisor.Module) error {
	sv, err := supervisor.New(modules,
		supervisor.WithLogger(r.logger),
		supervisor.WithPostInitHook("bus.start", r.bus.Start),
		supervisor.WithPreStopHook("bus.drain", r.bus.Drain),
		supervisor.WithPostStopHook("bus.shutdown", r.bus.Shutdown),
	)
	if err != nil {
		r.logger.Error("failed to create supervisor", "err", err)
		return err
	}
	r.sv = sv
	return nil
}

func (r *Runtime) close(ctx context.Context) {
	if r.db == nil {
		return
	}
	if err := r.db.Close(); err != nil {
		r.logger.ErrorContext(ctx, "failed to close sqlite database", "err", err)
	}
}

func newWebSearchTool(cfg search.Config) (tools.Tool, error) {
	if !cfg.Enabled() {
		return nil, nil
	}
	switch cfg.Provider {
	case search.ProviderTavily:
		client, err := search.NewTavily(cfg.Tavily)
		if err != nil {
			return nil, err
		}
		return web.NewSearch(client)
	default:
		return nil, fmt.Errorf("web_search: unsupported provider %q", cfg.Provider)
	}
}
