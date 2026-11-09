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

// DefaultURL is the public instance used when no URL is configured.
func DefaultURL(p Platform) string {
	if p == GitLab {
		return "https://gitlab.com"
	}
	return "https://github.com"
}

// PlatformName is the display name, e.g. "GitHub".
func PlatformName(p Platform) string {
	if p == GitLab {
		return "GitLab"
	}
	return "GitHub"
}

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
	Platform Platform
	Host     string // instance host, e.g. github.com or gitlab.archlinux.org
	Repo     string // owner/repo or group/subgroup/project
	RepoURL  string // web URL of the target repository
	Number   int
	Title    string
	URL      string
	Draft    bool

	BaseBranch  string // target branch
	HeadBranch  string // source branch
	HeadRepo    string // source repository (differs from Repo for forks); empty if unknown/deleted
	HeadRepoURL string
	UpdatedAt   time.Time

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
	return fmt.Sprintf("%s:%s/%s#%d", p.Platform, p.Host, p.Repo, p.Number)
}

// RepoLabel is the repository as shown in the table: prefixed with the host for self-hosted
// instances, so identically named projects on different instances can be told apart.
func (p *PR) RepoLabel() string {
	if p.Host == "" || "https://"+p.Host == DefaultURL(p.Platform) {
		return p.Repo
	}
	return p.Host + "/" + p.Repo
}

func (p *PR) HasNews() bool {
	return p.NewComments > 0 || p.NewReactions > 0 || p.BehindGrew
}
