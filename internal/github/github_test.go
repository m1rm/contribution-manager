package github

import "testing"

func TestNewEndpoints(t *testing.T) {
	for in, want := range map[string][3]string{
		"":                             {"github.com", "https://api.github.com", "https://api.github.com/graphql"},
		"https://github.com/":          {"github.com", "https://api.github.com", "https://api.github.com/graphql"},
		"https://ghe.example.com":      {"ghe.example.com", "https://ghe.example.com/api/v3", "https://ghe.example.com/api/graphql"},
		"https://example.com:8443/ghe": {"example.com:8443", "https://example.com:8443/ghe/api/v3", "https://example.com:8443/ghe/api/graphql"},
	} {
		c := New(in, "t")
		if got := [3]string{c.host, c.restAPI, c.graphql}; got != want {
			t.Errorf("New(%q) = %v, want %v", in, got, want)
		}
	}
}
