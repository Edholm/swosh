package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Edholm/swosh/internal/db/sqlc"
)

func TestRequestLang(t *testing.T) {
	tests := []struct {
		name, cookie, accept string
		want                 lang
	}{
		{"nothing", "", "", english},
		{"Swedish browser", "", "sv-SE,sv;q=0.9,en;q=0.8", swedish},
		{"Finland Swedish", "", "sv-FI", swedish},
		{"English first", "", "en-US,sv;q=0.5", english},
		{"Swedish as the fallback", "", "de-DE,sv;q=0.5", swedish},
		{"neither", "", "de-DE,fr;q=0.5", english},
		{"garbage", "", ";;;", english},
		{"cookie wins", "en", "sv-SE", english},
		{"cookie alone", "sv", "", swedish},
		{"unknown cookie", "de", "sv-SE", swedish},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.accept != "" {
				r.Header.Set("Accept-Language", tt.accept)
			}
			if tt.cookie != "" {
				r.AddCookie(&http.Cookie{Name: langCookie, Value: tt.cookie})
			}
			if got := requestLang(r); got != tt.want {
				t.Errorf("requestLang = %q, want %q", got, tt.want)
			}
		})
	}
}

// doIn is ts.do with the browser asking for l.
func (ts *testServer) doIn(t *testing.T, l lang, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.Header.Set("Accept-Language", string(l))
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

func TestSwedishPages(t *testing.T) {
	ts := newTestServer(t,
		sqlc.Swosh{ID: "abc123", Payee: "070-000 00 00", Amount: 100},
		sqlc.Swosh{ID: "forevr", Payee: "1230000000", Amount: 5, PayeeEditable: true, AmountEditable: true, MessageEditable: true},
	)
	for _, tt := range []struct {
		target string
		code   int
		want   []string
	}{
		{"/", http.StatusOK, []string{
			`<html lang="sv">`,
			`<title>Swosh – betallänkar för Swish</title>`,
			`<span>Skapa en länk</span>`,
			`"copied":"Kopierad!"`,
			`<a href="/lang?l=en" class="lang-switch" lang="en"`,
		}},
		{"/abc123", http.StatusOK, []string{
			`<meta property="og:title" content="Swisha 100 kr till 070-000 00 00"/>`,
			`<meta property="og:locale" content="sv_SE">`,
			`<dd>Aldrig</dd>`,
			`Du kontrollerar och bekräftar betalningen i Swish.</p>`,
		}},
		{"/forevr", http.StatusOK, []string{`där du kan ändra numret, beloppet och meddelandet.</p>`}},
		{"/nope", http.StatusNotFound, []string{`<h1>Länken finns inte</h1>`}},
		{"/login", http.StatusOK, []string{`<h1>Logga in som admin</h1>`}},
	} {
		rec := ts.doIn(t, swedish, http.MethodGet, tt.target, "")
		if rec.Code != tt.code {
			t.Errorf("GET %s = %d, want %d", tt.target, rec.Code, tt.code)
		}
		if got := rec.Header().Get("Vary"); got != "Accept-Language, Cookie" {
			t.Errorf("GET %s: Vary = %q", tt.target, got)
		}
		body := rec.Body.String()
		for _, want := range tt.want {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s is missing %s", tt.target, want)
			}
		}
	}

	// English pages offer Swedish.
	if body := ts.do(t, http.MethodGet, "/", "").Body.String(); !strings.Contains(body, `<a href="/lang?l=sv"`) {
		t.Error("English front page has no switch to Swedish")
	}
}

func TestCreateErrorInSwedish(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.doIn(t, swedish, http.MethodPost, "/api/create", `{"phone":"0700000000","amount":0}`)
	var got errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body, err)
	}
	if want := (errorResponse{Error: true, Reason: "Beloppet måste vara minst 1 kr, inte 0.", Field: "amount"}); got != want {
		t.Errorf("response = %+v, want %+v", got, want)
	}
}

func TestSetLang(t *testing.T) {
	ts := newTestServer(t)
	tests := []struct {
		name, target, referer, location, cookie string
	}{
		{"back to the page", "/lang?l=sv", "http://example.com/abc123?redirect=false", "/abc123?redirect=false", "sv"},
		{"English", "/lang?l=en", "http://example.com/", "/", "en"},
		{"no referer", "/lang?l=sv", "", "/", "sv"},
		{"other site", "/lang?l=sv", "https://evil.example/x", "/", "sv"},
		{"unknown language", "/lang?l=de", "http://example.com/abc123", "/abc123", ""},
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
				if c.Name == langCookie {
					got = c.Value
				}
			}
			if got != tt.cookie {
				t.Errorf("cookie = %q, want %q", got, tt.cookie)
			}
		})
	}
}

// TestMessages checks that every message has both languages with the same
// arguments, and that the templates only use messages that exist.
func TestMessages(t *testing.T) {
	verbs := regexp.MustCompile(`%[a-z]|\{\d\}`)
	for key, m := range messages {
		if m.en == "" || m.sv == "" {
			t.Errorf("%s is missing a language: %+v", key, m)
		}
		if en, sv := verbs.FindAllString(m.en, -1), verbs.FindAllString(m.sv, -1); strings.Join(en, " ") != strings.Join(sv, " ") {
			t.Errorf("%s has arguments %v in English but %v in Swedish", key, en, sv)
		}
	}

	used := regexp.MustCompile(`\{\{t "([^"]+)"`)
	files, err := fs.Glob(templateFiles, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := fs.ReadFile(templateFiles, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range used.FindAllStringSubmatch(string(b), -1) {
			if _, ok := messages[m[1]]; !ok {
				t.Errorf("%s uses unknown message %q", f, m[1])
			}
		}
	}
}
