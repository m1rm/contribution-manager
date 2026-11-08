# contribution-manager

A small, read-only CLI that shows **all your open pull requests on GitHub and merge requests on GitLab
in one colored terminal table** – so you don't have to log in to both platforms to check on your
contributions.

For every open PR/MR it tells you:

| Column | Meaning |
|---|---|
| `●` | Something changed since your last run (these rows are sorted to the top) |
| **Repository / #** | Where the PR lives. `#123` (GitHub) / `!45` (GitLab) are clickable in terminals that support OSC 8 links |
| **CI** | `✔` passed · `✘` failed · `●` running |
| **Review** | `approved` · `changes` requested · `pending` |
| **Comments** | Comments by *other people*; `+2 7` = 2 new since last run, 7 total |
| **Reactions** | Emoji reactions; `+N` marks new ones |
| **Last comment** | Who commented last and how long ago |
| **Upstream** | `✔ current`, `↓3 rebase` (target branch is 3 commits ahead), or `⚠ conflicts`; `new` if it moved since last run |
| **Updated** | Time since the PR was last updated |

It only ever *reads* from the APIs, authenticated with personal access tokens. Nothing is written to
GitHub or GitLab.

## Requirements

- [Go](https://go.dev/dl/) 1.25 or newer
- [just](https://github.com/casey/just) command runner

## Setup

### 1. Clone

```sh
git clone git@github.com:m1rm/contribution-manager.git
cd contribution-manager
```

### 2. Create personal access tokens

**GitHub** – <https://github.com/settings/tokens>

- *Classic token* with the `repo` scope (recommended): covers PRs in private repos of any org.
- *Fine-grained token* with read-only **Pull requests**, **Contents** and **Metadata**: works too, but is
  limited to the repos of a single owner, plus public repos.

**GitLab** – <https://gitlab.com/-/user_settings/personal_access_tokens>
(or the same page on your self-hosted instance)

- Scope: `read_api`

You can configure just one of the two platforms; the other one is then skipped.

### 3. Configure

```sh
cp .env.example .env
```

Edit `.env`:

```sh
PRWATCH_GITHUB_TOKEN=ghp_...
PRWATCH_GITLAB_TOKEN=glpat-...
# PRWATCH_GITLAB_URL=https://gitlab.example.com   # only for self-hosted GitLab
```

`just` loads `.env` automatically. `.env` is git-ignored, so your tokens stay local.
The variables are prefixed with `PRWATCH_` so they don't clash with a `GITHUB_TOKEN` that other tools
may already have set in your shell. `just` does not override variables that are already set.

### 4. Run

```sh
just
```

## Usage

```sh
just list               # GitHub + GitLab (same as: just list --all, or just `just`)
just list --github      # GitHub only   (shortcut: just github)
just list --gitlab      # GitLab only   (shortcut: just gitlab)
just list --urls        # add a column with full URLs
just list --width 70    # allow longer titles (default 48)
just peek               # show new activity without marking it as seen
just reset              # forget what was seen; the next run becomes the new baseline
```

Other recipes: `just build`, `just test`, `just fmt`, `just install` (installs the binary to `~/go/bin`),
`just clean`. Run `just --list` to see them all.

The binary can also be used directly: `./bin/prwatch list --github`.

### How "new" is detected

Every `list` run saves a snapshot of the comment, reaction and upstream counts per PR to
`~/.local/state/prwatch/state.json` (respects `$XDG_STATE_HOME`; override with `$PRWATCH_STATE_FILE`).
The next run compares against that snapshot. The very first run only records the baseline, so nothing
shows as new yet. Use `just peek` (`--keep`) if you want to look without resetting the "new" markers.

Colors and links are turned off automatically when the output is piped, or when `NO_COLOR` is set.

## Limitations

- GitHub: the last 100 comments/reviews and the first 50 review threads (50 comments each) per PR are
  counted. Reaction counts include your own reactions.
- GitLab: reactions are counted on the MR itself, not on individual comments.
- If one platform fails (e.g. an expired token), the other one is still shown, and the error is printed.

## Project layout

```
main.go                 CLI flags and orchestration
internal/github         GraphQL search + REST compare (behind-by) per PR
internal/gitlab         REST: MRs, divergence, pipeline, notes, award emoji, approvals
internal/state          "last seen" snapshot for new-activity detection
internal/render         colored table output (go-pretty)
justfile                convenience recipes
```

## License

GPL-3.0 – see [LICENSE](LICENSE).
