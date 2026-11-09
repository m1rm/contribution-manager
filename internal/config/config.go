// Package config reads the GitHub/GitLab instances to query from the environment.
//
// Each instance is a token plus an optional base URL. The unsuffixed variables configure the
// first instance; any number of further instances use a common suffix:
//
//	PRWATCH_GITLAB_TOKEN=...                         # gitlab.com
//	PRWATCH_GITLAB_TOKEN_ARCH=...                    # a second GitLab instance
//	PRWATCH_GITLAB_URL_ARCH=https://gitlab.archlinux.org
package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"prwatch/internal/model"
)

type Instance struct {
	Platform model.Platform
	Suffix   string // "" for the unsuffixed variables
	URL      string // normalized base URL, e.g. https://gitlab.com
	Token    string
}

// Host is the URL's host (with port, if any), used to label PRs and errors.
func (i Instance) Host() string {
	u, _ := url.Parse(i.URL)
	return u.Host
}

// Name is a human-readable label such as "GitLab (gitlab.archlinux.org)".
func (i Instance) Name() string {
	return fmt.Sprintf("%s (%s)", model.PlatformName(i.Platform), i.Host())
}

// TokenVar is the name of the variable holding this instance's token.
func (i Instance) TokenVar() string { return varName(i.Platform, "TOKEN", i.Suffix) }

var varRe = regexp.MustCompile(`^PRWATCH_(GITHUB|GITLAB)_(TOKEN|URL)(?:_([A-Za-z0-9_]+))?$`)

func varName(p model.Platform, kind, suffix string) string {
	n := "PRWATCH_" + strings.ToUpper(string(p)) + "_" + kind
	if suffix != "" {
		n += "_" + suffix
	}
	return n
}

// Load parses environ (as returned by os.Environ). It returns the configured instances, sorted by
// platform with the unsuffixed instance first, plus warnings about variables that are ignored.
func Load(environ []string) ([]Instance, []string, error) {
	type key struct {
		p      model.Platform
		suffix string
	}
	tokens := map[key]string{}
	urls := map[key]string{}
	for _, kv := range environ {
		name, val, _ := strings.Cut(kv, "=")
		m := varRe.FindStringSubmatch(name)
		if m == nil || strings.TrimSpace(val) == "" {
			continue
		}
		k := key{model.Platform(strings.ToLower(m[1])), m[3]}
		if m[2] == "TOKEN" {
			tokens[k] = strings.TrimSpace(val)
		} else {
			urls[k] = strings.TrimSpace(val)
		}
	}

	var warnings []string
	for k := range urls {
		if _, ok := tokens[k]; !ok {
			warnings = append(warnings, fmt.Sprintf("ignoring %s: %s is not set",
				varName(k.p, "URL", k.suffix), varName(k.p, "TOKEN", k.suffix)))
		}
	}
	sort.Strings(warnings)

	var out []Instance
	seen := map[string]string{} // platform+host -> token variable
	for k, tok := range tokens {
		raw := urls[k]
		if raw == "" {
			raw = model.DefaultURL(k.p)
		}
		norm, err := normalize(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", varName(k.p, "URL", k.suffix), err)
		}
		inst := Instance{Platform: k.p, Suffix: k.suffix, URL: norm, Token: tok}
		id := string(k.p) + " " + inst.Host()
		if other, dup := seen[id]; dup {
			a, b := other, inst.TokenVar()
			if b < a {
				a, b = b, a
			}
			return nil, nil, fmt.Errorf("%s and %s both point to %s – configure each instance only once", a, b, inst.Host())
		}
		seen[id] = inst.TokenVar()
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		return out[i].Suffix < out[j].Suffix // "" sorts first
	})
	return out, warnings, nil
}

// FromEnv is Load(os.Environ()).
func FromEnv() ([]Instance, []string, error) { return Load(os.Environ()) }

func normalize(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("invalid URL %q: scheme must be http or https", raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid URL %q: missing host", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid URL %q: must not contain a query or fragment", raw)
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}
