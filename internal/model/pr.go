// Package model holds the platform-independent representation of a pull/merge request.
package model

import (
	"fmt"
	"time"
)

type Platform string

const (
	GitHub Platform = "github"
	GitLab Platform = "gitlab"
)

type CIStatus string

const (
	CINone    CIStatus = ""
	CISuccess CIStatus = "success"
	CIFailed  CIStatus = "failed"
	CIRunning CIStatus = "running"
)

type ReviewStatus string

const (
	ReviewNone     ReviewStatus = ""
	ReviewPending  ReviewStatus = "pending"
	ReviewApproved ReviewStatus = "approved"
	ReviewChanges  ReviewStatus = "changes"
)

// BehindUnknown is used when the upstream comparison could not be determined.
const BehindUnknown = -1

type PR struct {
	Platform  Platform
	Repo      string // owner/repo or group/subgroup/project
	Number    int
	Title     string
	URL       string
	Draft     bool
	UpdatedAt time.Time

	// Activity by people other than the token owner.
	Comments      int
	Reactions     int
	LastCommentBy string
	LastCommentAt time.Time

	Behind    int // commits on the target branch not in the PR branch
	Conflicts bool
	CI        CIStatus
	Review    ReviewStatus

	// Filled in by comparing against the locally stored state.
	NewComments  int
	NewReactions int
	BehindGrew   bool
}

func (p *PR) Key() string {
	return fmt.Sprintf("%s:%s#%d", p.Platform, p.Repo, p.Number)
}

func (p *PR) HasNews() bool {
	return p.NewComments > 0 || p.NewReactions > 0 || p.BehindGrew
}
