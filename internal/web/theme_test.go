package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetTheme(t *testing.T) {
	ts := newTestServer(t)
	tests := []struct {
		name, target, referer, location, cookie string
	}{
		{"back to the page", "/theme?t=dark", "http://example.com/abc123?redirect=false", "/abc123?redirect=false", "dark"},
		{"light", "/theme?t=light", "http://example.com/", "/", "light"},
		{"other site", "/theme?t=dark", "https://evil.example/x", "/", "dark"},
		{"unknown theme", "/theme?t=pink", "http://example.com/abc123", "/abc123", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			rec := httptest.NewRecorder()
			ts.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.location {
				t.Errorf("got %d to %q, want 303 to %q", rec.Code, rec.Header().Get("Location"), tt.location)
			}
			var got string
			for _, c := range rec.Result().Cookies() {
				if c.Name == themeCookie {
					got = c.Value
				}
			}
			if got != tt.cookie {
				t.Errorf("cookie = %q, want %q", got, tt.cookie)
			}
		})
	}
}

func TestThemedPages(t *testing.T) {
	ts := newTestServer(t)
	tests := []struct {
		cookie        string
		want, notWant []string
	}{
		{"", []string{
			`<html lang="en">`,
			`<meta name="color-scheme" content="light dark">`,
			`<source srcset="/img/swish-logo-dark.png" media="(prefers-color-scheme: dark)">`,
			`href="/theme?t=dark"`,
			`href="/theme?t=light"`,
		}, []string{`data-theme`}},
		{"dark", []string{
			`<html lang="en" data-theme="dark">`,
			`<meta name="color-scheme" content="dark">`,
			`<img src="/img/swish-logo-dark.png"`,
			`href="/theme?t=light"`,
		}, []string{`href="/theme?t=dark"`, `swish-logo-light.png`}},
		{"light", []string{
			`<html lang="en" data-theme="light">`,
			`<meta name="color-scheme" content="light">`,
			`<img src="/img/swish-logo-light.png"`,
			`href="/theme?t=dark"`,
		}, []string{`href="/theme?t=light"`, `swish-logo-dark.png`}},
		{"pink", []string{`<html lang="en">`}, []string{`data-theme`}},
	}
	for _, tt := range tests {
		t.Run(tt.cookie, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: themeCookie, Value: tt.cookie})
			}
			rec := httptest.NewRecorder()
			ts.handler.ServeHTTP(rec, req)
			body := rec.Body.String()
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("page is missing %s", want)
				}
			}
			for _, nope := range tt.notWant {
				if strings.Contains(body, nope) {
					t.Errorf("page has %s", nope)
				}
			}
		})
	}
}
