package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/iamhectordev/hector/pkg/tools"
)

type toolsClient interface {
	GetIssueWithBlocking(context.Context, Repository, int) (IssueWithBlocking, error)
	CreateMilestone(context.Context, Repository, string, ...MilestoneOption) (Milestone, error)
	ListMilestones(context.Context, Repository, ...ListMilestonesOption) ([]Milestone, error)
	UpdateMilestone(context.Context, Repository, int, ...MilestoneOption) (Milestone, error)
	AddBlockedBy(context.Context, Repository, int, int) (IssueWithBlocking, error)
	RemoveBlockedBy(context.Context, Repository, int, int) (IssueWithBlocking, error)
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

// NewGetIssueTool returns a tool that fetches one GitHub issue with blocking info.
func NewGetIssueTool(client toolsClient) (tools.Tool, error) {
	return tools.New(
		"get_issue",
		"Returns one GitHub issue with blocked_by and blocks arrays. Requires repo as owner/name.",
		func(ctx context.Context, in getIssueInput) (IssueWithBlocking, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return IssueWithBlocking{}, err
			}
			if in.IssueNumber <= 0 {
				return IssueWithBlocking{}, fmt.Errorf("github: issue_number must be positive")
			}
			return client.GetIssueWithBlocking(ctx, repo, in.IssueNumber)
		},
	)
}

// NewCreateMilestoneTool returns a tool that creates a GitHub milestone.
func NewCreateMilestoneTool(client toolsClient) (tools.Tool, error) {
	return tools.New(
		"create_milestone",
		"Creates a GitHub milestone and returns the milestone JSON. Requires repo as owner/name.",
		func(ctx context.Context, in createMilestoneInput) (Milestone, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return Milestone{}, err
			}
			opts := []MilestoneOption{}
			if in.Description != nil {
				opts = append(opts, WithMilestoneDescription(*in.Description))
			}
			return client.CreateMilestone(ctx, repo, in.Title, opts...)
		},
	)
}

// NewListMilestonesTool returns a tool that lists GitHub milestones.
func NewListMilestonesTool(client toolsClient) (tools.Tool, error) {
	return tools.New(
		"list_milestones",
		"Returns GitHub milestones for a repository. Requires repo as owner/name; state may be open, closed, or all.",
		func(ctx context.Context, in listMilestonesInput) ([]Milestone, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return nil, err
			}
			opts := []ListMilestonesOption{}
			if in.State != "" {
				opts = append(opts, WithMilestoneState(in.State))
			}
			return client.ListMilestones(ctx, repo, opts...)
		},
	)
}

// NewUpdateMilestoneTool returns a tool that updates a GitHub milestone.
func NewUpdateMilestoneTool(client toolsClient) (tools.Tool, error) {
	return tools.New(
		"update_milestone",
		"Updates a GitHub milestone and returns the milestone JSON. Requires repo as owner/name.",
		func(ctx context.Context, in updateMilestoneInput) (Milestone, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return Milestone{}, err
			}
			opts := []MilestoneOption{}
			if in.Title != nil {
				opts = append(opts, WithMilestoneTitle(*in.Title))
			}
			if in.Description != nil {
				opts = append(opts, WithMilestoneDescription(*in.Description))
			}
			return client.UpdateMilestone(ctx, repo, in.MilestoneNumber, opts...)
		},
	)
}

// NewCreateBlockedByRelationshipTool returns a tool that creates a blocked-by relationship.
func NewCreateBlockedByRelationshipTool(client toolsClient) (tools.Tool, error) {
	return tools.New(
		"create_blocked_by_relationship",
		"Creates a GitHub blocked-by relationship and returns the updated blocked issue JSON. Requires repo as owner/name.",
		func(ctx context.Context, in blockedByRelationshipInput) (IssueWithBlocking, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return IssueWithBlocking{}, err
			}
			return client.AddBlockedBy(ctx, repo, in.BlockingIssueNumber, in.BlockedIssueNumber)
		},
	)
}

// NewRemoveBlockedByRelationshipTool returns a tool that removes a blocked-by relationship.
func NewRemoveBlockedByRelationshipTool(client toolsClient) (tools.Tool, error) {
	return tools.New(
		"remove_blocked_by_relationship",
		"Removes a GitHub blocked-by relationship and returns the updated blocked issue JSON. Requires repo as owner/name.",
		func(ctx context.Context, in blockedByRelationshipInput) (IssueWithBlocking, error) {
			repo, err := parseRepository(in.Repo)
			if err != nil {
				return IssueWithBlocking{}, err
			}
			return client.RemoveBlockedBy(ctx, repo, in.BlockingIssueNumber, in.BlockedIssueNumber)
		},
	)
}

func newTools(client toolsClient) ([]tools.Tool, error) {
	if client == nil {
		return nil, fmt.Errorf("github: tools client is required")
	}
	constructors := []func(toolsClient) (tools.Tool, error){
		NewGetIssueTool,
		NewCreateMilestoneTool,
		NewListMilestonesTool,
		NewUpdateMilestoneTool,
		NewCreateBlockedByRelationshipTool,
		NewRemoveBlockedByRelationshipTool,
	}
	out := make([]tools.Tool, 0, len(constructors))
	for _, constructor := range constructors {
		tool, err := constructor(client)
		if err != nil {
			return nil, err
		}
		out = append(out, tool)
	}
	return out, nil
}

func parseRepository(value string) (Repository, error) {
	owner, name, ok := strings.Cut(value, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repository{}, fmt.Errorf("github: repo must be owner/name")
	}
	return Repository{Owner: owner, Name: name}, nil
}
