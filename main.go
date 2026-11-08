// prwatch lists your open GitHub pull requests and GitLab merge requests in one table,
// highlighting new comments/reactions since the last run and PRs that are behind upstream.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/jedib0t/go-pretty/v6/text"

	"prwatch/internal/github"
	"prwatch/internal/gitlab"
	"prwatch/internal/model"
	"prwatch/internal/render"
	"prwatch/internal/state"
)

const usage = `prwatch – your open PRs/MRs on GitHub and GitLab

Usage:
  prwatch list [--all | --github | --gitlab] [--keep] [--urls] [--width N]
  prwatch reset          forget what was seen (next run becomes the new baseline)
  prwatch state-path     print where the seen-state is stored

Environment:
  PRWATCH_GITHUB_TOKEN   GitHub personal access token
  PRWATCH_GITLAB_TOKEN   GitLab personal access token (scope: read_api)
  PRWATCH_GITLAB_URL     GitLab base URL (default: https://gitlab.com)
  NO_COLOR               disable colors
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "list", "ls":
		err = runList(os.Args[2:])
	case "reset":
		err = state.Reset()
		if err == nil {
			fmt.Println("State cleared.")
		}
	case "state-path":
		fmt.Println(state.Path())
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, text.FgRed.Sprint("error:"), err)
		os.Exit(1)
	}
}

type source struct {
	platform model.Platform
	name     string
	token    string
	fetch    func(context.Context) ([]*model.PR, error)
}

type result struct {
	prs []*model.PR
	err error
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	all := fs.Bool("all", false, "show GitHub and GitLab (default)")
	onlyGH := fs.Bool("github", false, "only GitHub")
	onlyGL := fs.Bool("gitlab", false, "only GitLab")
	keep := fs.Bool("keep", false, "don't mark activity as seen")
	urls := fs.Bool("urls", false, "add a column with full URLs")
	width := fs.Int("width", 48, "max title width")
	fs.Parse(args)

	plain := os.Getenv("NO_COLOR") != "" || !isTerminal()
	if plain {
		text.DisableColors()
	}

	ghToken := os.Getenv("PRWATCH_GITHUB_TOKEN")
	glToken := os.Getenv("PRWATCH_GITLAB_TOKEN")
	sources := []source{
		{model.GitHub, "GitHub", ghToken, github.New(ghToken).OpenPRs},
		{model.GitLab, "GitLab", glToken, gitlab.New(os.Getenv("PRWATCH_GITLAB_URL"), glToken).OpenMRs},
	}

	explicit := *onlyGH || *onlyGL
	if *all {
		explicit = false
	}
	var active []source
	for _, s := range sources {
		wanted := !explicit || (s.platform == model.GitHub && *onlyGH) || (s.platform == model.GitLab && *onlyGL)
		if !wanted {
			continue
		}
		if s.token == "" {
			envName := "PRWATCH_" + map[model.Platform]string{model.GitHub: "GITHUB", model.GitLab: "GITLAB"}[s.platform] + "_TOKEN"
			if explicit {
				return fmt.Errorf("%s requested but %s is not set", s.name, envName)
			}
			fmt.Fprintln(os.Stderr, text.Faint.Sprintf("skipping %s: %s not set", s.name, envName))
			continue
		}
		active = append(active, s)
	}
	if len(active) == 0 {
		return fmt.Errorf("no tokens configured – set PRWATCH_GITHUB_TOKEN and/or PRWATCH_GITLAB_TOKEN (see .env.example)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	results := make([]result, len(active))
	var wg sync.WaitGroup
	for i, s := range active {
		wg.Go(func() {
			prs, err := s.fetch(ctx)
			results[i] = result{prs, err}
		})
	}
	if isTerminal() {
		fmt.Fprint(os.Stderr, text.Faint.Sprint("fetching…"))
	}
	wg.Wait()
	if isTerminal() {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}

	st, err := state.Load()
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	var prs []*model.PR
	failed := 0
	for i, r := range results {
		if r.err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "%s %s: %v\n", text.FgRed.Sprint("✘"), active[i].name, r.err)
			continue
		}
		st.Annotate(r.prs)
		prs = append(prs, r.prs...)
		if !*keep {
			st.Record(active[i].platform, r.prs)
		}
	}
	if failed == len(active) {
		return fmt.Errorf("all sources failed")
	}

	render.Sort(prs)
	render.Table(os.Stdout, prs, render.Options{ShowURLs: *urls, TitleWidth: *width, Plain: plain})

	if !*keep {
		if err := st.Save(); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}
	}
	return nil
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
