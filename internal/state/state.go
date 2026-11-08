// Package state remembers what was seen on the last run, so new comments/reactions can be highlighted.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"prwatch/internal/model"
)

type Entry struct {
	Comments  int       `json:"comments"`
	Reactions int       `json:"reactions"`
	Behind    int       `json:"behind"`
	SeenAt    time.Time `json:"seen_at"`
}

type State struct {
	path    string
	Entries map[string]Entry `json:"entries"`
}

func Path() string {
	if p := os.Getenv("PRWATCH_STATE_FILE"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "prwatch", "state.json")
}

func Load() (*State, error) {
	s := &State{path: Path(), Entries: map[string]Entry{}}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Entries == nil {
		s.Entries = map[string]Entry{}
	}
	return s, nil
}

// Annotate fills the New* fields of each PR by diffing against the stored snapshot.
// PRs never seen before are treated as a baseline (nothing is "new").
func (s *State) Annotate(prs []*model.PR) {
	for _, pr := range prs {
		e, ok := s.Entries[pr.Key()]
		if !ok {
			continue
		}
		pr.NewComments = max(0, pr.Comments-e.Comments)
		pr.NewReactions = max(0, pr.Reactions-e.Reactions)
		pr.BehindGrew = e.Behind >= 0 && pr.Behind > e.Behind
	}
}

// Record stores the current snapshot for the given platform, dropping PRs that are no longer open.
func (s *State) Record(p model.Platform, prs []*model.PR) {
	prefix := string(p) + ":"
	for k := range s.Entries {
		if strings.HasPrefix(k, prefix) {
			delete(s.Entries, k)
		}
	}
	now := time.Now()
	for _, pr := range prs {
		s.Entries[pr.Key()] = Entry{Comments: pr.Comments, Reactions: pr.Reactions, Behind: pr.Behind, SeenAt: now}
	}
}

func (s *State) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func Reset() error {
	err := os.Remove(Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
