package render

import (
	"encoding/json"
	"io"
	"time"

	"prwatch/internal/model"
)

// JSONError reports an instance that could not be fetched.
type JSONError struct {
	Instance string `json:"instance"`
	Error    string `json:"error"`
}

type jsonPR struct {
	Platform    model.Platform `json:"platform"`
	Host        string         `json:"host"`
	Repo        string         `json:"repo"`
	RepoURL     string         `json:"repo_url"`
	Number      int            `json:"number"`
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Draft       bool           `json:"draft"`
	UpdatedAt   time.Time      `json:"updated_at"`
	BaseBranch  string         `json:"base_branch"`
	HeadBranch  string         `json:"head_branch"`
	HeadRepo    string         `json:"head_repo"`
	HeadRepoURL string         `json:"head_repo_url"`
	Behind      *int           `json:"behind"` // null when unknown
	Conflicts   bool           `json:"conflicts"`
	CI          string         `json:"ci"`     // success, failed, running, or ""
	Review      string         `json:"review"` // approved, changes, pending, or ""

	Comments      int        `json:"comments"`
	NewComments   int        `json:"new_comments"`
	Reactions     int        `json:"reactions"`
	NewReactions  int        `json:"new_reactions"`
	LastCommentBy string     `json:"last_comment_by,omitempty"`
	LastCommentAt *time.Time `json:"last_comment_at,omitempty"`
	BehindGrew    bool       `json:"behind_grew"`
}

// JSON writes {"prs": [...], "errors": [...]} for scripts and agents.
func JSON(w io.Writer, prs []*model.PR, errs []JSONError) error {
	out := struct {
		PRs    []jsonPR    `json:"prs"`
		Errors []JSONError `json:"errors"`
	}{PRs: make([]jsonPR, 0, len(prs)), Errors: errs}
	if out.Errors == nil {
		out.Errors = []JSONError{}
	}
	for _, p := range prs {
		j := jsonPR{
			Platform: p.Platform, Host: p.Host, Repo: p.Repo, RepoURL: p.RepoURL,
			Number: p.Number, Title: p.Title, URL: p.URL, Draft: p.Draft, UpdatedAt: p.UpdatedAt,
			BaseBranch: p.BaseBranch, HeadBranch: p.HeadBranch, HeadRepo: p.HeadRepo, HeadRepoURL: p.HeadRepoURL,
			Conflicts: p.Conflicts, CI: string(p.CI), Review: string(p.Review),
			Comments: p.Comments, NewComments: p.NewComments, Reactions: p.Reactions, NewReactions: p.NewReactions,
			LastCommentBy: p.LastCommentBy, BehindGrew: p.BehindGrew,
		}
		if p.Behind != model.BehindUnknown {
			b := p.Behind
			j.Behind = &b
		}
		if !p.LastCommentAt.IsZero() {
			t := p.LastCommentAt
			j.LastCommentAt = &t
		}
		out.PRs = append(out.PRs, j)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
