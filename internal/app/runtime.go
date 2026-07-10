package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/internal/embed"
	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/modules/agent"
	emailmodule "github.com/iamhectordev/hector/modules/email"
	memorymod "github.com/iamhectordev/hector/modules/memory"
	"github.com/iamhectordev/hector/integrations"
	slackintegration "github.com/iamhectordev/hector/integrations/slack"
	toolsmod "github.com/iamhectordev/hector/modules/tools"
	githubtools "github.com/iamhectordev/hector/modules/tools/github"
	"github.com/iamhectordev/hector/modules/tools/web"
	pkgtools "github.com/iamhectordev/hector/pkg/tools"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm"
	memorysqlite "github.com/iamhectordev/hector/pkg/memory/sqlite"
	"github.com/iamhectordev/hector/pkg/safehttp"
	sessionsqlite "github.com/iamhectordev/hector/pkg/session/sqlite"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
)

// Runtime wires and runs the long-running Hector application.
type Runtime struct {
	cfg     *Config
	profile Profile
	logger  *slog.Logger

	tracing      *tracing.Runtime
	db           *sql.DB
	bus          *waffle.EventBus
	sv           *supervisor.Supervisor
	githubCloser io.Closer
}

// NewRuntime builds a Runtime from typed application config.
func NewRuntime(cfg *Config, opts ...Option) (*Runtime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("app: config is required")
	}
	r := &Runtime{
		cfg:     cfg,
		profile: ProfileServe,
		logger:  slog.Default().With("component", "runtime"),
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
	ctx = telem.WithLogger(ctx, r.logger)
	telem.Logger(ctx).InfoContext(ctx, "starting app runtime")
	defer r.close(ctx)

	if err := r.initTracing(ctx); err != nil {
		return err
	}
	if err := r.init(ctx); err != nil {
		return err
	}

	rep := r.sv.Run(ctx)
	telem.Logger(ctx).InfoContext(ctx, "app runtime finished",
		telem.Any("reason", rep.Reason),
		telem.String("trigger_module", rep.TriggerModule),
		telem.Any("signal", rep.Signal),
	)
	return rep.Err()
}

func (r *Runtime) initTracing(ctx context.Context) error {
	tracingRuntime, err := tracing.Setup(ctx, r.cfg.Tracing)
	if err != nil {
		return err
	}
	r.tracing = tracingRuntime
	return nil
}

func (r *Runtime) init(ctx context.Context) error {
	if err := r.initDatabase(ctx); err != nil {
		return err
	}
	if err := r.initBus(ctx); err != nil {
		return err
	}

	completer, err := llm.New(ctx, &r.cfg.LLM)
	if err != nil {
		return err
	}
	webSearch, err := newWebSearchTool(r.cfg.WebSearch)
	if err != nil {
		return err
	}
	surfaces, surfaceReplyHandlers, err := r.initSurfaces()
	if err != nil {
		return err
	}
	integrationHosts, integrationReplyHandlers, err := r.initIntegrations()
	if err != nil {
		return err
	}
	replyHandlers := append(surfaceReplyHandlers, integrationReplyHandlers...)
	memStore, err := r.initMemoryStore()
	if err != nil {
		return err
	}
	toolRegistry, toolsModule, err := r.initTools(replyHandlers, webSearch, memStore)
	if err != nil {
		return err
	}
	modules, err := r.initModules(ctx, completer, toolRegistry, toolsModule, append(surfaces, integrationHosts...), memStore)
	if err != nil {
		return err
	}
	return r.initSupervisor(ctx, modules)
}

func (r *Runtime) initDatabase(ctx context.Context) error {
	db, err := dbsqlite.Open(ctx, r.cfg.DB)
	if err != nil {
		telem.Logger(ctx).ErrorContext(ctx, "failed to open sqlite database", telem.Any("err", err))
		return err
	}
	r.db = db

	if err := dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations(), sessionsqlite.Migrations(), memorysqlite.Migrations()); err != nil {
		telem.Logger(ctx).ErrorContext(ctx, "failed to migrate sqlite database", telem.Any("err", err))
		return fmt.Errorf("sqlite migrations: %w", err)
	}
	return nil
}

func (r *Runtime) initBus(ctx context.Context) error {
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithLogger(r.logger),
		waffle.WithStore(wafflesqlite.NewStore(r.db)),
		waffle.WithPersistentReactions(),
	)
	if err != nil {
		telem.Logger(ctx).ErrorContext(ctx, "failed to create event bus", telem.Any("err", err))
		return err
	}
	r.bus = bus
	return nil
}

func (r *Runtime) initSurfaces() ([]supervisor.Module, []comms.ReplyHandler, error) {
	switch r.profile {
	case ProfileServe:
		return nil, nil, nil
	case ProfileChat:
		return []supervisor.Module{tui.NewModule(r.bus)}, []comms.ReplyHandler{tui.NewReplyHandler(nil)}, nil
	default:
		return nil, nil, fmt.Errorf("app: unsupported profile %q", r.profile)
	}
}

func (r *Runtime) initIntegrations() ([]supervisor.Module, []comms.ReplyHandler, error) {
	var hosts []supervisor.Module
	var replyHandlers []comms.ReplyHandler

	if r.cfg.Slack.Enabled {
		slk, err := slackintegration.New(r.bus, &r.cfg.Slack)
		if err != nil {
			return nil, nil, err
		}
		if sf, ok := any(slk).(integrations.Surface); ok {
			replyHandlers = append(replyHandlers, sf.ReplyHandler())
		}
		host, err := integrations.NewHost(slk)
		if err != nil {
			return nil, nil, err
		}
		hosts = append(hosts, host)
	}

	if r.profile == ProfileServe && len(hosts) == 0 {
		return nil, nil, fmt.Errorf("serve requires at least one enabled integration")
	}

	return hosts, replyHandlers, nil
}

func (r *Runtime) initTools(	replyHandlers []comms.ReplyHandler, webSearch pkgtools.Tool, memStore *memorysqlite.Store) (*pkgtools.Registry, *toolsmod.Module, error) {
	replyRouter, err := comms.NewReplyRouter(replyHandlers...)
	if err != nil {
		return nil, nil, err
	}
	timeNow, err := pkgtools.NewTimeNow()
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
	memRecall, err := toolsmod.NewMemRecall(memStore)
	if err != nil {
		return nil, nil, err
	}
	toolRegistry, err := pkgtools.NewRegistry(replyRouter, timeNow, webFetch, webSearch, memRecall)
	if err != nil {
		return nil, nil, err
	}
	toolsModule, err := toolsmod.NewModule(r.bus, toolRegistry)
	if err != nil {
		return nil, nil, err
	}
	return toolRegistry, toolsModule, nil
}

func (r *Runtime) initModules(
	ctx context.Context,
	completer llm.Completer,
	toolRegistry *pkgtools.Registry,
	toolsModule *toolsmod.Module,
	surfaces []supervisor.Module,
	memStore *memorysqlite.Store,
) ([]supervisor.Module, error) {
	modules := []supervisor.Module{}
	if r.cfg.GitHub.Enabled {
		githubCloser, err := githubtools.Register(ctx, r.cfg.GitHub, toolRegistry)
		if err != nil {
			return nil, err
		}
		r.githubCloser = githubCloser
	}

	sessionStore := sessionsqlite.NewStore(r.db)
	loop := agent.NewLoop(completer,
		agent.WithTools(toolRegistry),
	)
	var perceiver agent.Perceiver
	var err error
	if r.cfg.Agent.Perception.Enabled {
		perceiver, err = agent.NewStructuredPerceiver(completer)
		if err != nil {
			return nil, err
		}
	}
	memMod, err := memorymod.NewModule(r.bus, memStore, sessionStore, completer)
	if err != nil {
		return nil, err
	}
	modules, err = r.appendEmailModule(modules)
	if err != nil {
		return nil, err
	}
	modules = append(modules,
		agent.NewModule(r.bus, loop,
			agent.WithConfig(r.cfg.Agent),
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithPerceiver(perceiver),
			agent.WithSessionStore(sessionStore),
		),
		toolsModule,
		memMod,
	)
	modules = append(modules, surfaces...)
	return modules, nil
}

func (r *Runtime) appendEmailModule(modules []supervisor.Module) ([]supervisor.Module, error) {
	if !r.cfg.Email.Enabled {
		return modules, nil
	}
	emailModule, err := emailmodule.NewModule(r.cfg.Email, emailmodule.NewNoopMailbox())
	if err != nil {
		return nil, err
	}
	return append(modules, emailModule), nil
}

func (r *Runtime) initSupervisor(ctx context.Context, modules []supervisor.Module) error {
	opts := []supervisor.Option{
		supervisor.WithLogger(r.logger),
		supervisor.WithPostInitHook("bus.start", r.bus.Start),
		supervisor.WithPreStopHook("bus.drain", r.bus.Drain),
		supervisor.WithPostStopHook("bus.shutdown", r.bus.Shutdown),
	}
	if r.tracing != nil {
		opts = append(opts, supervisor.WithPostStopHook("tracing.shutdown", func(ctx context.Context) error {
			if err := r.tracing.Shutdown(ctx); err != nil {
				return err
			}
			r.tracing = nil
			return nil
		}))
	}
	sv, err := supervisor.New(modules, opts...)
	if err != nil {
		telem.Logger(ctx).ErrorContext(ctx, "failed to create supervisor", telem.Any("err", err))
		return err
	}
	r.sv = sv
	return nil
}

func (r *Runtime) close(ctx context.Context) {
	if r.githubCloser != nil {
		if err := r.githubCloser.Close(); err != nil {
			telem.Logger(ctx).ErrorContext(ctx, "failed to close github mcp client", telem.Any("err", err))
		}
	}
	if r.db != nil {
		if err := r.db.Close(); err != nil {
			telem.Logger(ctx).ErrorContext(ctx, "failed to close sqlite database", telem.Any("err", err))
		}
	}
	if r.tracing != nil {
		if err := r.tracing.Shutdown(ctx); err != nil {
			telem.Logger(ctx).ErrorContext(ctx, "failed to shut down tracing", telem.Any("err", err))
		}
	}
}

func (r *Runtime) initMemoryStore() (*memorysqlite.Store, error) {
	if !r.cfg.Memory.EmbedEnabled {
		return memorysqlite.NewStore(r.db), nil
	}
	embedder, err := embed.New(r.cfg.Memory.Embed)
	if err != nil {
		return nil, fmt.Errorf("memory: embedder: %w", err)
	}
	return memorysqlite.NewStore(r.db, memorysqlite.WithEmbedder(embedder)), nil
}

func newWebSearchTool(cfg search.Config) (pkgtools.Tool, error) {
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
