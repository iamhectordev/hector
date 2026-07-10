package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/iamhectordev/hector/internal/github"
	pkgtools "github.com/iamhectordev/hector/pkg/tools"
)

type toolsClient interface {
	GetIssueWithBlocking(context.Context, gh.Repository, int) (gh.IssueWithBlocking, error)
	CreateMilestone(context.Context, gh.Repository, string, ...gh.MilestoneOption) (gh.Milestone, error)
	ListMilestones(context.Context, gh.Repository, ...gh.ListMilestonesOption) ([]gh.Milestone, error)
	UpdateMilestone(context.Context, gh.Repository, int, ...gh.MilestoneOption) (gh.Milestone, error)
	AddBlockedBy(context.Context, gh.Repository, int, int) (gh.IssueWithBlocking, error)
	RemoveBlockedBy(context.Context, gh.Repository, int, int) (gh.IssueWithBlocking, error)
}

type getIssueInput struct {
	Repo        string `json:"repo" jsonschema:"repository in owner/name form"`
	IssueNumber int    `json:"issue_number" jsonschema:"issue number to fetch"`
}

type createMilestoneInput struct {
	Repo        string  `json:"repo" jsonschema:"repository in owner/name form"`
	Title       string  `json:"title" jsonschema:"milestone title"`
	Description *string `json:"description,omitempty" jsonschema:"optional milestone description"`
}

type listMilestonesInput struct {
	Repo  string `json:"repo" jsonschema:"repository in owner/name form"`
	State string `json:"state,omitempty" jsonschema:"optional milestone state: open, closed, or all"`
}

type updateMilestoneInput struct {
	Repo            string  `json:"repo" jsonschema:"repository in owner/name form"`
	MilestoneNumber int     `json:"milestone_number" jsonschema:"milestone number to update"`
	Title           *string `json:"title,omitempty" jsonschema:"optional replacement title"`
	Description     *string `json:"description,omitempty" jsonschema:"optional replacement description"`
}

type blockedByRelationshipInput struct {
	Repo                string `json:"repo" jsonschema:"repository in owner/name form"`
	BlockingIssueNumber int    `json:"blocking_issue_number" jsonschema:"issue number that blocks the other issue"`
	BlockedIssueNumber  int    `json:"blocked_issue_number" jsonschema:"issue number that is blocked"`
}

func NewGetIssueTool(client toolsClient) (pkgtools.Tool, error) {
	return pkgtools.New(
		"get_issue",
		"Returns one GitHub issue with blocked_by and blocks arrays. Requires repo as owner/name.",
		func(ctx context.Context, in getIssueInput) (gh.IssueWithBlocking, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return gh.IssueWithBlocking{}, err
			}
			if in.IssueNumber <= 0 {
				return gh.IssueWithBlocking{}, fmt.Errorf("github: issue_number must be positive")
			}
			return client.GetIssueWithBlocking(ctx, repo, in.IssueNumber)
		},
	)
}

func NewCreateMilestoneTool(client toolsClient) (pkgtools.Tool, error) {
	return pkgtools.New(
		"create_milestone",
		"Creates a GitHub milestone and returns the milestone JSON. Requires repo as owner/name.",
		func(ctx context.Context, in createMilestoneInput) (gh.Milestone, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return gh.Milestone{}, err
			}
			opts := []gh.MilestoneOption{}
			if in.Description != nil {
				opts = append(opts, gh.WithMilestoneDescription(*in.Description))
			}
			return client.CreateMilestone(ctx, repo, in.Title, opts...)
		},
	)
}

func NewListMilestonesTool(client toolsClient) (pkgtools.Tool, error) {
	return pkgtools.New(
		"list_milestones",
		"Returns GitHub milestones for a repository. Requires repo as owner/name; state may be open, closed, or all.",
		func(ctx context.Context, in listMilestonesInput) ([]gh.Milestone, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return nil, err
			}
			opts := []gh.ListMilestonesOption{}
			if in.State != "" {
				opts = append(opts, gh.WithMilestoneState(in.State))
			}
			return client.ListMilestones(ctx, repo, opts...)
		},
	)
}

func NewUpdateMilestoneTool(client toolsClient) (pkgtools.Tool, error) {
	return pkgtools.New(
		"update_milestone",
		"Updates a GitHub milestone and returns the milestone JSON. Requires repo as owner/name.",
		func(ctx context.Context, in updateMilestoneInput) (gh.Milestone, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return gh.Milestone{}, err
			}
			opts := []gh.MilestoneOption{}
			if in.Title != nil {
				opts = append(opts, gh.WithMilestoneTitle(*in.Title))
			}
			if in.Description != nil {
				opts = append(opts, gh.WithMilestoneDescription(*in.Description))
			}
			return client.UpdateMilestone(ctx, repo, in.MilestoneNumber, opts...)
		},
	)
}

func NewCreateBlockedByRelationshipTool(client toolsClient) (pkgtools.Tool, error) {
	return pkgtools.New(
		"create_blocked_by_relationship",
		"Creates a GitHub blocked-by relationship and returns the updated blocked issue JSON. Requires repo as owner/name.",
		func(ctx context.Context, in blockedByRelationshipInput) (gh.IssueWithBlocking, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return gh.IssueWithBlocking{}, err
			}
			return client.AddBlockedBy(ctx, repo, in.BlockingIssueNumber, in.BlockedIssueNumber)
		},
	)
}

func NewRemoveBlockedByRelationshipTool(client toolsClient) (pkgtools.Tool, error) {
	return pkgtools.New(
		"remove_blocked_by_relationship",
		"Removes a GitHub blocked-by relationship and returns the updated blocked issue JSON. Requires repo as owner/name.",
		func(ctx context.Context, in blockedByRelationshipInput) (gh.IssueWithBlocking, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return gh.IssueWithBlocking{}, err
			}
			return client.RemoveBlockedBy(ctx, repo, in.BlockingIssueNumber, in.BlockedIssueNumber)
		},
	)
}

// NewTools returns all GitHub tools. Returns an error if client is nil.
func NewTools(client toolsClient) ([]pkgtools.Tool, error) {
	if client == nil {
		return nil, fmt.Errorf("github: tools client is required")
	}
	constructors := []func(toolsClient) (pkgtools.Tool, error){
		NewGetIssueTool,
		NewCreateMilestoneTool,
		NewListMilestonesTool,
		NewUpdateMilestoneTool,
		NewCreateBlockedByRelationshipTool,
		NewRemoveBlockedByRelationshipTool,
	}
	out := make([]pkgtools.Tool, 0, len(constructors))
	for _, constructor := range constructors {
		tool, err := constructor(client)
		if err != nil {
			return nil, err
		}
		out = append(out, tool)
	}
	return out, nil
}

func parseRepository(value string) (gh.Repository, error) {
	owner, name, ok := strings.Cut(value, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return gh.Repository{}, fmt.Errorf("github: repo must be owner/name")
	}
	return gh.Repository{Owner: owner, Name: name}, nil
}
