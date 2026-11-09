// Package render prints PRs as a colored table.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"prwatch/internal/model"
)

type Options struct {
	ShowURLs   bool
	Plain      bool // no colors / terminal hyperlinks (e.g. when piped)
	TitleWidth int
	Now        time.Time
}

var (
	cGitHub = text.Colors{text.FgHiWhite, text.Bold}
	cGitLab = text.Colors{text.FgHiYellow, text.Bold}
	cDim    = text.Colors{text.Faint}
	cGood   = text.Colors{text.FgGreen}
	cWarn   = text.Colors{text.FgYellow}
	cBad    = text.Colors{text.FgRed}
	cNew    = text.Colors{text.FgHiMagenta, text.Bold}
	cHeader = text.Colors{text.FgHiCyan, text.Bold}
)

// Sort puts PRs with news first, then the most recently updated.
func Sort(prs []*model.PR) {
	sort.SliceStable(prs, func(i, j int) bool {
		if prs[i].HasNews() != prs[j].HasNews() {
			return prs[i].HasNews()
		}
		return prs[i].UpdatedAt.After(prs[j].UpdatedAt)
	})
}

func Table(w io.Writer, prs []*model.PR, o Options) {
	if o.TitleWidth == 0 {
		o.TitleWidth = 48
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.Style().Color.Header = cHeader
	t.Style().Color.Border = cDim
	t.Style().Color.Separator = cDim
	t.Style().Format.Header = text.FormatDefault

	header := table.Row{"", "Repository", "#", "Title", "CI", "Review", "Comments", "Reactions", "Last comment", "Upstream", "Updated"}
	if o.ShowURLs {
		header = append(header, "URL")
	}
	t.AppendHeader(header)

	var news, rebase int
	for _, pr := range prs {
		if pr.HasNews() {
			news++
		}
		if pr.Behind > 0 || pr.Conflicts {
			rebase++
		}

		marker := " "
		if pr.HasNews() {
			marker = cNew.Sprint("●")
		}
		platform := cGitHub.Sprint("GH")
		if pr.Platform == model.GitLab {
			platform = cGitLab.Sprint("GL")
		}
		numPrefix := "#"
		if pr.Platform == model.GitLab {
			numPrefix = "!"
		}

		title := truncate(pr.Title, o.TitleWidth)
		if pr.Draft {
			title = cDim.Sprint("[draft] " + truncate(pr.Title, o.TitleWidth-8))
		}

		lastBy := cDim.Sprint("—")
		if pr.LastCommentBy != "" {
			lastBy = pr.LastCommentBy + cDim.Sprint(" · "+ago(o.Now, pr.LastCommentAt))
		}

		row := table.Row{
			marker + " " + platform,
			pr.RepoLabel(),
			link(o, pr.URL, fmt.Sprintf("%s%d", numPrefix, pr.Number)),
			title,
			ci(pr.CI),
			review(pr.Review),
			counter(pr.Comments, pr.NewComments),
			counter(pr.Reactions, pr.NewReactions),
			lastBy,
			upstream(pr),
			cDim.Sprint(ago(o.Now, pr.UpdatedAt)),
		}
		if o.ShowURLs {
			row = append(row, cDim.Sprint(pr.URL))
		}
		t.AppendRow(row)
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 3, Align: text.AlignRight},
		{Number: 5, Align: text.AlignCenter},
		{Number: 7, Align: text.AlignRight},
		{Number: 8, Align: text.AlignRight},
	})

	if len(prs) == 0 {
		fmt.Fprintln(w, cDim.Sprint("No open pull/merge requests. 🎉"))
		return
	}
	t.Render()

	summary := fmt.Sprintf("%d open", len(prs))
	if news > 0 {
		summary += " · " + cNew.Sprintf("%d with new activity", news)
	}
	if rebase > 0 {
		summary += " · " + cWarn.Sprintf("%d could be rebased", rebase)
	}
	fmt.Fprintln(w, " "+summary)
}

func link(o Options, url, label string) string {
	if o.Plain {
		return label
	}
	return text.Hyperlink(url, label)
}

func counter(total, fresh int) string {
	if fresh > 0 {
		return cNew.Sprintf("+%d", fresh) + " " + fmt.Sprint(total)
	}
	if total == 0 {
		return cDim.Sprint("0")
	}
	return fmt.Sprint(total)
}

func ci(s model.CIStatus) string {
	switch s {
	case model.CISuccess:
		return cGood.Sprint("✔")
	case model.CIFailed:
		return cBad.Sprint("✘")
	case model.CIRunning:
		return cWarn.Sprint("●")
	}
	return cDim.Sprint("—")
}

func review(s model.ReviewStatus) string {
	switch s {
	case model.ReviewApproved:
		return cGood.Sprint("approved")
	case model.ReviewChanges:
		return cBad.Sprint("changes")
	case model.ReviewPending:
		return cDim.Sprint("pending")
	}
	return cDim.Sprint("—")
}

func upstream(pr *model.PR) string {
	var s string
	switch {
	case pr.Conflicts:
		s = cBad.Sprint("⚠ conflicts")
		if pr.Behind > 0 {
			s += cBad.Sprintf(" (↓%d)", pr.Behind)
		}
	case pr.Behind > 0:
		s = cWarn.Sprintf("↓%d rebase", pr.Behind)
	case pr.Behind == 0:
		s = cGood.Sprint("✔ current")
	default:
		s = cDim.Sprint("?")
	}
	if pr.BehindGrew {
		s += " " + cNew.Sprint("new")
	}
	return s
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if n <= 1 || len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}

func ago(now, t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}
