package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	insts, warns, err := Load([]string{
		"PRWATCH_GITLAB_TOKEN=a",
		"PRWATCH_GITLAB_TOKEN_ARCH=b",
		"PRWATCH_GITLAB_URL_ARCH=gitlab.archlinux.org/",
		"PRWATCH_GITHUB_TOKEN_WORK=c",
		"PRWATCH_GITHUB_URL_WORK=https://GHE.example.com",
		"PRWATCH_GITHUB_URL_ORPHAN=https://x.example.com",
		"PRWATCH_GITHUB_TOKEN=", // empty: ignored
		"GITHUB_TOKEN=unrelated",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(insts))
	for i, in := range insts {
		got[i] = string(in.Platform) + "|" + in.Suffix + "|" + in.URL + "|" + in.Token
	}
	want := []string{
		"github|WORK|https://ghe.example.com|c",
		"gitlab||https://gitlab.com|a",
		"gitlab|ARCH|https://gitlab.archlinux.org|b",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("instances:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "PRWATCH_GITHUB_URL_ORPHAN") {
		t.Errorf("warnings = %v", warns)
	}
	if insts[2].Name() != "GitLab (gitlab.archlinux.org)" || insts[2].TokenVar() != "PRWATCH_GITLAB_TOKEN_ARCH" {
		t.Errorf("name/var = %q %q", insts[2].Name(), insts[2].TokenVar())
	}
}

func TestLoadErrors(t *testing.T) {
	for name, env := range map[string][]string{
		"duplicate host": {"PRWATCH_GITLAB_TOKEN=a", "PRWATCH_GITLAB_TOKEN_2=b", "PRWATCH_GITLAB_URL_2=https://gitlab.com/"},
		"bad scheme":     {"PRWATCH_GITHUB_TOKEN=a", "PRWATCH_GITHUB_URL=ftp://example.com"},
	} {
		if _, _, err := Load(env); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if insts, _, err := Load(nil); err != nil || len(insts) != 0 {
		t.Errorf("empty env: %v %v", insts, err)
	}
}
