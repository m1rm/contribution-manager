// Package httpx contains a tiny JSON-over-HTTP helper shared by the API clients.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

var Client = &http.Client{Timeout: 30 * time.Second}

// Do sends a request with the given headers and decodes a JSON response into out (if non-nil).
// The raw response is returned so callers can inspect headers (e.g. pagination).
func Do(ctx context.Context, method, url string, headers map[string]string, body, out any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "prwatch")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode >= 300 {
		msg := string(data)
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return resp, &StatusError{Code: resp.StatusCode, URL: url, Body: msg}
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp, fmt.Errorf("decoding %s: %w", url, err)
		}
	}
	return resp, nil
}

type StatusError struct {
	Code int
	URL  string
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("HTTP %d from %s: %s", e.Code, e.URL, e.Body)
}

// ForEach runs fn for every index in [0,n) with at most `limit` concurrent goroutines.
func ForEach(n, limit int, fn func(i int)) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range n {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			fn(i)
		})
	}
	wg.Wait()
}
