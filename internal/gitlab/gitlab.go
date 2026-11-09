// Package gitlab fetches the token owner's open merge requests from GitLab (gitlab.com or self-hosted).
package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"prwatch/internal/httpx"
	"prwatch/internal/model"
)

type Client struct {
	base  string // e.g. https://gitlab.com
	host  string // e.g. gitlab.com
	token string
}

func New(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = model.DefaultURL(model.GitLab)
	}
	base := strings.TrimRight(baseURL, "/")
	host := base
	if u, err := url.Parse(base); err == nil && u.Host != "" {
		host = u.Host
	}
	return &Client{base: base, host: host, token: token}
}

func (c *Client) get(ctx context.Context, path string, out any) (*http.Response, error) {
	return httpx.Do(ctx, http.MethodGet, c.base+"/api/v4"+path,
		map[string]string{"PRIVATE-TOKEN": c.token}, nil, out)
}

type mrSummary struct {
	IID          int       `json:"iid"`
	ProjectID    int       `json:"project_id"`
	Title        string    `json:"title"`
	WebURL       string    `json:"web_url"`
	Draft        bool      `json:"draft"`
	UpdatedAt    time.Time `json:"updated_at"`
	HasConflicts bool      `json:"has_conflicts"`
	References   struct {
		Full string `json:"full"`
	} `json:"references"`
}

type mrDetail struct {
	DivergedCommitsCount *int `json:"diverged_commits_count"`
	HasConflicts         bool `json:"has_conflicts"`
	HeadPipeline         *struct {
		Status string `json:"status"`
	} `json:"head_pipeline"`
}

type note struct {
	System    bool      `json:"system"`
	CreatedAt time.Time `json:"created_at"`
	Author    struct {
		Username string `json:"username"`
	} `json:"author"`
}

// OpenMRs returns all open merge requests authored by the token owner.
func (c *Client) OpenMRs(ctx context.Context) ([]*model.PR, error) {
	var me struct {
		Username string `json:"username"`
	}
	if _, err := c.get(ctx, "/user", &me); err != nil {
		return nil, err
	}

	var mrs []mrSummary
	for page := "1"; page != ""; {
		var batch []mrSummary
		resp, err := c.get(ctx, "/merge_requests?scope=created_by_me&state=opened&per_page=100&page="+page, &batch)
		if err != nil {
			return nil, err
		}
		mrs = append(mrs, batch...)
		page = resp.Header.Get("X-Next-Page")
	}

	prs := make([]*model.PR, len(mrs))
	httpx.ForEach(len(mrs), 6, func(i int) {
		prs[i] = c.enrich(ctx, mrs[i], me.Username)
	})
	return prs, nil
}

func (c *Client) enrich(ctx context.Context, m mrSummary, me string) *model.PR {
	repo, _, _ := strings.Cut(m.References.Full, "!")
	pr := &model.PR{
		Platform:  model.GitLab,
		Host:      c.host,
		Repo:      repo,
		Number:    m.IID,
		Title:     m.Title,
		URL:       m.WebURL,
		Draft:     m.Draft,
		UpdatedAt: m.UpdatedAt,
		Conflicts: m.HasConflicts,
		Behind:    model.BehindUnknown,
	}
	mrPath := fmt.Sprintf("/projects/%d/merge_requests/%d", m.ProjectID, m.IID)

	// Upstream divergence + pipeline status.
	var d mrDetail
	if _, err := c.get(ctx, mrPath+"?include_diverged_commits_count=true", &d); err == nil {
		if d.DivergedCommitsCount != nil {
			pr.Behind = *d.DivergedCommitsCount
		}
		pr.Conflicts = d.HasConflicts
		if d.HeadPipeline != nil {
			switch d.HeadPipeline.Status {
			case "success":
				pr.CI = model.CISuccess
			case "failed", "canceled":
				pr.CI = model.CIFailed
			case "running", "pending", "created", "preparing", "waiting_for_resource", "scheduled":
				pr.CI = model.CIRunning
			}
		}
	}

	// Comments by others (system notes like "added 1 commit" are skipped).
	for page := "1"; page != ""; {
		var notes []note
		resp, err := c.get(ctx, mrPath+"/notes?per_page=100&page="+page, &notes)
		if err != nil {
			break
		}
		for _, n := range notes {
			if n.System || strings.EqualFold(n.Author.Username, me) {
				continue
			}
			pr.Comments++
			if n.CreatedAt.After(pr.LastCommentAt) {
				pr.LastCommentAt = n.CreatedAt
				pr.LastCommentBy = n.Author.Username
			}
		}
		page = resp.Header.Get("X-Next-Page")
	}

	// Emoji reactions on the MR itself; X-Total gives the count without fetching them all.
	if resp, err := c.get(ctx, mrPath+"/award_emoji?per_page=1", nil); err == nil {
		pr.Reactions, _ = strconv.Atoi(resp.Header.Get("X-Total"))
	}

	// Approval state.
	var appr struct {
		Approved          bool       `json:"approved"`
		ApprovalsRequired int        `json:"approvals_required"`
		ApprovedBy        []struct{} `json:"approved_by"`
	}
	if _, err := c.get(ctx, mrPath+"/approvals", &appr); err == nil {
		switch {
		case len(appr.ApprovedBy) > 0 && appr.Approved:
			pr.Review = model.ReviewApproved
		case appr.ApprovalsRequired > 0:
			pr.Review = model.ReviewPending
		}
	}
	return pr
}
