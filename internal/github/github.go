// Package github fetches the token owner's open pull requests from GitHub.
package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"prwatch/internal/httpx"
	"prwatch/internal/model"
)

type Client struct {
	host    string // e.g. github.com
	restAPI string // e.g. https://api.github.com
	graphql string // e.g. https://api.github.com/graphql
	token   string
}

// New creates a client for github.com or a GitHub Enterprise Server instance, given its web URL
// (e.g. https://github.com or https://github.example.com). An empty URL means github.com.
func New(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = model.DefaultURL(model.GitHub)
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || u.Host == "" {
		u, _ = url.Parse(model.DefaultURL(model.GitHub))
	}
	c := &Client{host: u.Host, token: token}
	switch strings.ToLower(u.Host) {
	case "github.com", "www.github.com", "api.github.com":
		c.host = "github.com"
		c.restAPI = "https://api.github.com"
		c.graphql = "https://api.github.com/graphql"
	default: // GitHub Enterprise Server
		c.restAPI = u.String() + "/api/v3"
		c.graphql = u.String() + "/api/graphql"
	}
	return c
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"Authorization":        "Bearer " + c.token,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
}

const searchQuery = `
query($q: String!, $after: String) {
  viewer { login }
  search(query: $q, type: ISSUE, first: 25, after: $after) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        number title url isDraft updatedAt
        repository { nameWithOwner }
        baseRefName headRefOid mergeable reviewDecision
        reactions { totalCount }
        comments(last: 100) {
          nodes { author { login } createdAt reactions { totalCount } }
        }
        reviews(last: 100) {
          nodes { author { login } body state submittedAt }
        }
        reviewThreads(first: 50) {
          nodes { comments(first: 50) { nodes { author { login } createdAt reactions { totalCount } } } }
        }
        commits(last: 1) {
          nodes { commit { statusCheckRollup { state } } }
        }
      }
    }
  }
}`

type actor struct{ Login string }

type comment struct {
	Author    *actor
	CreatedAt time.Time
	Reactions struct{ TotalCount int }
}

type prNode struct {
	Number         int
	Title          string
	URL            string
	IsDraft        bool
	UpdatedAt      time.Time
	Repository     struct{ NameWithOwner string }
	BaseRefName    string
	HeadRefOid     string
	Mergeable      string
	ReviewDecision string
	Reactions      struct{ TotalCount int }
	Comments       struct{ Nodes []comment }
	Reviews        struct {
		Nodes []struct {
			Author      *actor
			Body        string
			State       string
			SubmittedAt *time.Time
		}
	}
	ReviewThreads struct {
		Nodes []struct {
			Comments struct{ Nodes []comment }
		}
	}
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct{ State string }
			}
		}
	}
}

type searchResp struct {
	Data struct {
		Viewer struct{ Login string }
		Search struct {
			PageInfo struct {
				HasNextPage bool
				EndCursor   string
			}
			Nodes []prNode
		}
	}
	Errors []struct{ Message string }
}

// OpenPRs returns all open PRs authored by the token owner.
func (c *Client) OpenPRs(ctx context.Context) ([]*model.PR, error) {
	var (
		prs   []*model.PR
		nodes []prNode
		login string
		after *string
	)
	for {
		var resp searchResp
		body := map[string]any{
			"query": searchQuery,
			"variables": map[string]any{
				"q":     "is:pr is:open author:@me archived:false sort:updated-desc",
				"after": after,
			},
		}
		if _, err := httpx.Do(ctx, http.MethodPost, c.graphql, c.headers(), body, &resp); err != nil {
			return nil, err
		}
		if len(resp.Errors) > 0 {
			msgs := make([]string, len(resp.Errors))
			for i, e := range resp.Errors {
				msgs[i] = e.Message
			}
			return nil, fmt.Errorf("github graphql: %s", strings.Join(msgs, "; "))
		}
		login = resp.Data.Viewer.Login
		for _, n := range resp.Data.Search.Nodes {
			if n.Number != 0 { // skip non-PR search hits
				nodes = append(nodes, n)
			}
		}
		if !resp.Data.Search.PageInfo.HasNextPage {
			break
		}
		cursor := resp.Data.Search.PageInfo.EndCursor
		after = &cursor
	}

	for _, n := range nodes {
		pr := convert(n, login)
		pr.Host = c.host
		prs = append(prs, pr)
	}

	// How far behind the base branch is each PR? One REST call per PR, in parallel.
	httpx.ForEach(len(nodes), 8, func(i int) {
		prs[i].Behind = c.behindBy(ctx, nodes[i])
	})
	return prs, nil
}

func convert(n prNode, me string) *model.PR {
	pr := &model.PR{
		Platform:  model.GitHub,
		Repo:      n.Repository.NameWithOwner,
		Number:    n.Number,
		Title:     n.Title,
		URL:       n.URL,
		Draft:     n.IsDraft,
		UpdatedAt: n.UpdatedAt,
		Conflicts: n.Mergeable == "CONFLICTING",
		Reactions: n.Reactions.TotalCount,
		Behind:    model.BehindUnknown,
	}

	isOther := func(a *actor) bool { return a == nil || !strings.EqualFold(a.Login, me) }
	seen := func(a *actor, at time.Time) {
		pr.Comments++
		if at.After(pr.LastCommentAt) {
			pr.LastCommentAt = at
			pr.LastCommentBy = "ghost"
			if a != nil {
				pr.LastCommentBy = a.Login
			}
		}
	}

	for _, cm := range n.Comments.Nodes {
		pr.Reactions += cm.Reactions.TotalCount
		if isOther(cm.Author) {
			seen(cm.Author, cm.CreatedAt)
		}
	}
	for _, t := range n.ReviewThreads.Nodes {
		for _, cm := range t.Comments.Nodes {
			pr.Reactions += cm.Reactions.TotalCount
			if isOther(cm.Author) {
				seen(cm.Author, cm.CreatedAt)
			}
		}
	}
	for _, r := range n.Reviews.Nodes {
		// Reviews without a body in COMMENTED state are just containers for inline comments.
		hasContent := strings.TrimSpace(r.Body) != "" || r.State == "APPROVED" || r.State == "CHANGES_REQUESTED"
		if hasContent && isOther(r.Author) && r.SubmittedAt != nil {
			seen(r.Author, *r.SubmittedAt)
		}
	}

	switch n.ReviewDecision {
	case "APPROVED":
		pr.Review = model.ReviewApproved
	case "CHANGES_REQUESTED":
		pr.Review = model.ReviewChanges
	case "REVIEW_REQUIRED":
		pr.Review = model.ReviewPending
	}

	if len(n.Commits.Nodes) > 0 {
		if r := n.Commits.Nodes[0].Commit.StatusCheckRollup; r != nil {
			switch r.State {
			case "SUCCESS":
				pr.CI = model.CISuccess
			case "FAILURE", "ERROR":
				pr.CI = model.CIFailed
			case "PENDING", "EXPECTED":
				pr.CI = model.CIRunning
			}
		}
	}
	return pr
}

// behindBy compares the base branch with the PR head commit. Comparing against the head SHA
// (instead of fork:branch) works for forks too, because GitHub keeps PR heads in the base repo.
func (c *Client) behindBy(ctx context.Context, n prNode) int {
	segs := strings.Split(n.BaseRefName, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	u := fmt.Sprintf("%s/repos/%s/compare/%s...%s?per_page=1",
		c.restAPI, n.Repository.NameWithOwner, strings.Join(segs, "/"), n.HeadRefOid)
	var resp struct {
		BehindBy int `json:"behind_by"`
	}
	if _, err := httpx.Do(ctx, http.MethodGet, u, c.headers(), nil, &resp); err != nil {
		return model.BehindUnknown
	}
	return resp.BehindBy
}
