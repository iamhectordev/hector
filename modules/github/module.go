package github

import (
	"context"
	"fmt"
	"log/slog"
)

const (
	verifyIssueOwner  = "iamhectordev"
	verifyIssueRepo   = "hector"
	verifyIssueNumber = 1
)

type issueReader interface {
	GetIssue(context.Context, Repository, int) (Issue, error)
}

// Module verifies the GitHub integration during application startup.
type Module struct {
	client issueReader
	logger *slog.Logger
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

// NewModule validates config and initializes the GitHub client.
func NewModule(cfg Config, opts ...ModuleOption) (*Module, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	m := &Module{
		client: client,
		logger: slog.Default().With("component", "module", "module", "github"),
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
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m *Module) Stop(context.Context) error {
	return nil
}
