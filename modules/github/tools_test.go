package github_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	hectorgithub "github.com/iamhectordev/hector/modules/github"
	"github.com/stretchr/testify/require"
)

func TestGetIssueToolRejectsRepoWithoutOwnerAndName(t *testing.T) {
	t.Parallel()

	tool, err := hectorgithub.NewGetIssueTool(&githubToolClient{})
	require.NoError(t, err)

	output, err := tool.Run(t.Context(), json.RawMessage(`{"repo":"hector","issue_number":28}`))
	require.NoError(t, err)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &envelope))
	require.Equal(t, "error", envelope["status"])
	require.Contains(t, envelope["message"], "owner/name")
}

func TestGitHubToolsRunAgainstClient(t *testing.T) {
	t.Parallel()

	client := &githubToolClient{}
	getIssue, err := hectorgithub.NewGetIssueTool(client)
	require.NoError(t, err)
	createMilestone, err := hectorgithub.NewCreateMilestoneTool(client)
	require.NoError(t, err)
	listMilestones, err := hectorgithub.NewListMilestonesTool(client)
	require.NoError(t, err)
	updateMilestone, err := hectorgithub.NewUpdateMilestoneTool(client)
	require.NoError(t, err)
	addBlockedBy, err := hectorgithub.NewCreateBlockedByRelationshipTool(client)
	require.NoError(t, err)
	removeBlockedBy, err := hectorgithub.NewRemoveBlockedByRelationshipTool(client)
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		tool func(context.Context) (string, error)
	}{
		{"get_issue", func(ctx context.Context) (string, error) {
			return getIssue.Run(ctx, json.RawMessage(`{"repo":"acme/widgets","issue_number":7}`))
		}},
		{"create_milestone", func(ctx context.Context) (string, error) {
			return createMilestone.Run(ctx, json.RawMessage(`{"repo":"acme/widgets","title":"v2","description":"Second"}`))
		}},
		{"list_milestones", func(ctx context.Context) (string, error) {
			return listMilestones.Run(ctx, json.RawMessage(`{"repo":"acme/widgets","state":"all"}`))
		}},
		{"update_milestone", func(ctx context.Context) (string, error) {
			return updateMilestone.Run(ctx, json.RawMessage(`{"repo":"acme/widgets","milestone_number":2,"title":"v2.1","description":"Updated"}`))
		}},
		{"create_blocked_by_relationship", func(ctx context.Context) (string, error) {
			return addBlockedBy.Run(ctx, json.RawMessage(`{"repo":"acme/widgets","blocking_issue_number":6,"blocked_issue_number":7}`))
		}},
		{"remove_blocked_by_relationship", func(ctx context.Context) (string, error) {
			return removeBlockedBy.Run(ctx, json.RawMessage(`{"repo":"acme/widgets","blocking_issue_number":6,"blocked_issue_number":7}`))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.tool(t.Context())
			require.NoError(t, err)
			var envelope map[string]any
			require.NoError(t, json.Unmarshal([]byte(output), &envelope))
			require.Equal(t, "ok", envelope["status"])
		})
	}

	require.Equal(t, []string{
		"get acme/widgets#7",
		"create milestone acme/widgets v2 opts:1",
		"list milestones acme/widgets opts:1",
		"update milestone acme/widgets #2 opts:2",
		"add blocked_by acme/widgets 6->7",
		"remove blocked_by acme/widgets 6->7",
	}, client.calls)
}

type githubToolClient struct {
	calls []string
}

func (c *githubToolClient) GetIssueWithBlocking(_ context.Context, repo hectorgithub.Repository, number int) (hectorgithub.IssueWithBlocking, error) {
	c.calls = append(c.calls, "get "+repo.Owner+"/"+repo.Name+"#"+strconv.Itoa(number))
	return hectorgithub.IssueWithBlocking{Issue: hectorgithub.Issue{Number: number, Title: "Issue"}}, nil
}

func (c *githubToolClient) CreateMilestone(_ context.Context, repo hectorgithub.Repository, title string, opts ...hectorgithub.MilestoneOption) (hectorgithub.Milestone, error) {
	c.calls = append(c.calls, "create milestone "+repo.Owner+"/"+repo.Name+" "+title+" opts:"+strconv.Itoa(len(opts)))
	return hectorgithub.Milestone{Number: 2, Title: title}, nil
}

func (c *githubToolClient) ListMilestones(_ context.Context, repo hectorgithub.Repository, opts ...hectorgithub.ListMilestonesOption) ([]hectorgithub.Milestone, error) {
	c.calls = append(c.calls, "list milestones "+repo.Owner+"/"+repo.Name+" opts:"+strconv.Itoa(len(opts)))
	return []hectorgithub.Milestone{{Number: 1, Title: "v1"}}, nil
}

func (c *githubToolClient) UpdateMilestone(_ context.Context, repo hectorgithub.Repository, number int, opts ...hectorgithub.MilestoneOption) (hectorgithub.Milestone, error) {
	c.calls = append(c.calls, "update milestone "+repo.Owner+"/"+repo.Name+" #"+strconv.Itoa(number)+" opts:"+strconv.Itoa(len(opts)))
	return hectorgithub.Milestone{Number: number}, nil
}

func (c *githubToolClient) AddBlockedBy(_ context.Context, repo hectorgithub.Repository, blockingIssueNumber int, blockedIssueNumber int) (hectorgithub.IssueWithBlocking, error) {
	c.calls = append(c.calls, "add blocked_by "+repo.Owner+"/"+repo.Name+" "+strconv.Itoa(blockingIssueNumber)+"->"+strconv.Itoa(blockedIssueNumber))
	return hectorgithub.IssueWithBlocking{Issue: hectorgithub.Issue{Number: blockedIssueNumber}}, nil
}

func (c *githubToolClient) RemoveBlockedBy(_ context.Context, repo hectorgithub.Repository, blockingIssueNumber int, blockedIssueNumber int) (hectorgithub.IssueWithBlocking, error) {
	c.calls = append(c.calls, "remove blocked_by "+repo.Owner+"/"+repo.Name+" "+strconv.Itoa(blockingIssueNumber)+"->"+strconv.Itoa(blockedIssueNumber))
	return hectorgithub.IssueWithBlocking{Issue: hectorgithub.Issue{Number: blockedIssueNumber}}, nil
}
