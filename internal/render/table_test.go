package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"prwatch/internal/model"
)

func TestTable(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	prs := []*model.PR{
		{Platform: model.GitHub, Repo: "golang/go", Number: 12345, Title: "cmd/go: add a very long title that definitely needs truncating somewhere",
			URL: "https://github.com/golang/go/pull/12345", UpdatedAt: now.Add(-2 * time.Hour),
			Comments: 7, NewComments: 2, Reactions: 3, LastCommentBy: "rsc", LastCommentAt: now.Add(-3 * time.Hour),
			Behind: 14, BehindGrew: true, CI: model.CISuccess, Review: model.ReviewChanges},
		{Platform: model.GitLab, Repo: "gitlab-org/gitlab", Number: 999, Title: "Fix pipeline flakiness",
			URL: "https://gitlab.com/gitlab-org/gitlab/-/merge_requests/999", UpdatedAt: now.Add(-5 * 24 * time.Hour),
			Behind: 0, CI: model.CIRunning, Review: model.ReviewApproved, Draft: true},
		{Platform: model.GitHub, Repo: "me/dotfiles", Number: 3, Title: "Conflicting change",
			URL: "https://github.com/me/dotfiles/pull/3", UpdatedAt: now.Add(-40 * 24 * time.Hour),
			Behind: 2, Conflicts: true, CI: model.CIFailed},
	}
	Sort(prs)
	var buf bytes.Buffer
	Table(&buf, prs, Options{Now: now})
	out := buf.String()
	t.Log("\n" + out)

	for _, want := range []string{"golang/go", "+2", "↓14 rebase", "conflicts", "3 open", "rsc"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	prs := []*model.PR{
		{Platform: model.GitLab, Host: "gitlab.example.com", Repo: "g/p", Number: 7, BaseBranch: "main", HeadBranch: "fix", Behind: 3},
		{Platform: model.GitHub, Host: "github.com", Repo: "o/r", Number: 1, Behind: model.BehindUnknown},
	}
	if err := JSON(&buf, prs, nil); err != nil {
		t.Fatal(err)
	}
	var out struct {
		PRs []struct {
			Behind     *int   `json:"behind"`
			BaseBranch string `json:"base_branch"`
			HeadBranch string `json:"head_branch"`
		} `json:"prs"`
		Errors []JSONError `json:"errors"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.PRs) != 2 || out.PRs[0].Behind == nil || *out.PRs[0].Behind != 3 || out.PRs[1].Behind != nil {
		t.Errorf("behind not encoded as expected: %s", buf.String())
	}
	if out.PRs[0].BaseBranch != "main" || out.PRs[0].HeadBranch != "fix" || out.Errors == nil {
		t.Errorf("unexpected output: %s", buf.String())
	}
}
