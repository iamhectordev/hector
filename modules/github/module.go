package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/internal/mcp"
	"github.com/iamhectordev/hector/modules/tools"
)

const (
	verifyIssueOwner  = "iamhectordev"
	verifyIssueRepo   = "hector"
	verifyIssueNumber = 1
)

type issueReader interface {
	GetIssue(context.Context, Repository, int) (Issue, error)
}

type toolRegistrar interface {
	Register(tools.Tool) error
}

// Module verifies the GitHub integration during application startup.
type Module struct {
	client    issueReader
	tokens    TokenProvider
	mcpConfig MCPConfig
	mcpClient *mcp.Client
	tools     toolRegistrar
	logger    *slog.Logger
}

// ModuleOption customizes a Module.
type ModuleOption func(*Module)

// WithLogger replaces the logger used by the module.
func WithLogger(logger *slog.Logger) ModuleOption {
	return func(m *Module) {
		if logger != nil {
			m.logger = logger
		}
	}
}

func WithToolRegistrar(registrar toolRegistrar) ModuleOption {
	return func(m *Module) {
		m.tools = registrar
	}
}

// NewModule validates config and initializes the GitHub client.
func NewModule(cfg Config, opts ...ModuleOption) (*Module, error) {
	tokenProvider, err := NewTokenProvider(cfg)
	if err != nil {
		return nil, err
	}
	client, err := NewClientWithTokenProvider(ClientConfig{APIURL: cfg.APIURL}, tokenProvider)
	if err != nil {
		return nil, err
	}
	m := &Module{
		client:    client,
		tokens:    tokenProvider,
		mcpConfig: cfg.MCP,
		logger:    slog.Default().With("component", "module", "module", "github"),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func (m *Module) Name() string {
	return "github"
}

func (m *Module) Init(ctx context.Context) error {
	repo := Repository{Owner: verifyIssueOwner, Name: verifyIssueRepo}
	issue, err := m.client.GetIssue(ctx, repo, verifyIssueNumber)
	if err != nil {
		return fmt.Errorf("github: verify issue read: %w", err)
	}
	m.logger.InfoContext(ctx, "github integration verified",
		"owner", repo.Owner,
		"repo", repo.Name,
		"issue_number", issue.Number,
		"issue_title", issue.Title,
	)
	if err := m.initMCP(ctx); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m *Module) Stop(context.Context) error {
	if m.mcpClient != nil {
		return m.mcpClient.Close()
	}
	return nil
}

func (m *Module) initMCP(ctx context.Context) error {
	if !m.mcpConfig.Configured() {
		return nil
	}
	if m.tools == nil {
		return fmt.Errorf("github: mcp configured without tool registrar")
	}
	token, err := m.tokens.Token(ctx)
	if err != nil {
		return err
	}
	client, err := mcp.NewClient(m.mcpConfig.config(token))
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	m.mcpClient = client
	discovered, err := client.ListTools(ctx)
	if err != nil {
		return err
	}
	for _, discoveredTool := range discovered {
		tool, err := tools.NewMCPTool("github", client, discoveredTool)
		if err != nil {
			return err
		}
		if err := m.tools.Register(tool); err != nil {
			return err
		}
	}
	m.logger.InfoContext(ctx, "github mcp tools registered", "count", len(discovered))
	return nil
}
