package web

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/Edholm/swosh/internal/auth"
	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
	"github.com/Edholm/swosh/internal/metrics"
)

func (f *fakeStore) ListSwoshes(context.Context) ([]sqlc.Swosh, error) {
	if f.err != nil {
		return nil, f.err
	}
	return slices.SortedFunc(maps.Values(f.links), func(a, b sqlc.Swosh) int { return cmp.Compare(a.ID, b.ID) }), nil
}

// UpdateSwosh fails with a unique violation when renaming onto a taken id,
// like Postgres.
func (f *fakeStore) UpdateSwosh(_ context.Context, arg sqlc.UpdateSwoshParams) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	sw, ok := f.links[arg.ID]
	if !ok {
		return 0, nil
	}
	if _, taken := f.links[arg.NewID]; taken && arg.NewID != arg.ID {
		return 0, &pgconn.PgError{Code: "23505"}
	}
	delete(f.links, arg.ID)
	sw.ID, sw.Payee, sw.Amount, sw.Description = arg.NewID, arg.Payee, arg.Amount, arg.Description
	f.links[sw.ID] = sw
	return 1, nil
}

func (f *fakeStore) DeleteSwosh(_ context.Context, id string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if _, ok := f.links[id]; !ok {
		return 0, nil
	}
	delete(f.links, id)
	return 1, nil
}

func (f *fakeStore) GetUserByUsername(_ context.Context, name string) (sqlc.User, error) {
	if f.err != nil {
		return sqlc.User{}, f.err
	}
	u, ok := f.users[name]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	return u, nil
}

const password = "s3cret"

// addUser adds an enabled user whose password is password.
func (ts *testServer) addUser(t *testing.T, name string, authorities ...string) {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	ts.store.users[name] = sqlc.User{Username: name, Password: "{bcrypt}" + string(h), Enabled: true, Authorities: authorities}
}

// client is a browser with a cookie jar of one cookie.
type client struct {
	ts     *testServer
	cookie *http.Cookie
	header http.Header
}

func (ts *testServer) client() *client {
	return &client{ts: ts, header: http.Header{}}
}

func (c *client) do(t *testing.T, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	}
	for k, v := range c.header {
		req.Header[k] = v
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.ts.handler.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == sessionCookie {
			c.cookie = ck
			if ck.MaxAge < 0 {
				c.cookie = nil
			}
		}
	}
	return rec
}

// login logs in as name and returns the CSRF token from the admin page.
func (c *client) login(t *testing.T, name string) string {
	t.Helper()
	rec := c.do(t, http.MethodPost, "/login", url.Values{"username": {name}, "password": {password}})
	if rec.Code != http.StatusSeeOther || c.cookie == nil {
		t.Fatalf("login as %s: %d, cookie %v", name, rec.Code, c.cookie)
	}
	rec = c.do(t, http.MethodGet, "/admin", nil)
	m := regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)">`).FindStringSubmatch(rec.Body.String())
	if m == nil {
		return ""
	}
	return m[1]
}

var allAuthorities = auth.AdminAuthorities

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, code, rec.Body)
	}
}

func wantLocation(t *testing.T, rec *httptest.ResponseRecorder, loc string) {
	t.Helper()
	if got := rec.Header().Get("Location"); got != loc {
		t.Errorf("Location = %q, want %q", got, loc)
	}
}

func TestAdminRequiresLogin(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	c := ts.client()

	c.header.Set("Accept", "text/html,application/xhtml+xml")
	rec := c.do(t, http.MethodGet, "/admin/abc123", nil)
	wantStatus(t, rec, http.StatusFound)
	wantLocation(t, rec, "/login?next=%2Fadmin%2Fabc123")

	c.header.Del("Accept")
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := c.do(t, method, "/admin/abc123", nil)
		wantStatus(t, rec, http.StatusUnauthorized)
		if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Basic ") {
			t.Errorf("%s: WWW-Authenticate = %q, want a Basic challenge", method, got)
		}
	}
	if _, ok := ts.store.links["abc123"]; !ok {
		t.Error("anonymous DELETE removed the link")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "admin", allAuthorities...)
	ts.addUser(t, "off", allAuthorities...)
	off := ts.store.users["off"]
	off.Enabled = false
	ts.store.users["off"] = off

	for _, tt := range []struct{ name, user, password string }{
		{"wrong password", "admin", "nope"},
		{"unknown user", "nobody", password},
		{"disabled user", "off", password},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := ts.client()
			rec := c.do(t, http.MethodPost, "/login", url.Values{"username": {tt.user}, "password": {tt.password}})
			wantStatus(t, rec, http.StatusUnauthorized)
			if c.cookie != nil {
				t.Error("got a session cookie")
			}
			if !strings.Contains(rec.Body.String(), "Bad credentials") {
				t.Error("login page does not say why")
			}

			c.header.Set("Authorization", "Basic "+basic(tt.user, tt.password))
			wantStatus(t, c.do(t, http.MethodGet, "/admin", nil), http.StatusUnauthorized)
		})
	}
}

func basic(user, pass string) string {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.SetBasicAuth(user, pass)
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Basic ")
}

func TestLoginRedirects(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "admin", allAuthorities...)
	for next, want := range map[string]string{
		"":                      "/admin",
		"/admin/abc123":         "/admin/abc123",
		"//evil.example":        "/admin",
		"/\\evil.example":       "/admin",
		"/\t/evil.example":      "/admin",
		"https://evil.example/": "/admin",
	} {
		rec := ts.client().do(t, http.MethodPost, "/login", url.Values{"username": {"admin"}, "password": {password}, "next": {next}})
		wantStatus(t, rec, http.StatusSeeOther)
		wantLocation(t, rec, want)
	}
}

func TestSessionCookie(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	c.login(t, "admin")
	if !c.cookie.HttpOnly || !c.cookie.Secure || c.cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie = %+v, want HttpOnly, Secure and SameSite=Lax", c.cookie)
	}

	tampered := *c.cookie
	tampered.Value = strings.Replace(tampered.Value, ".", "x.", 1)
	c2 := ts.client()
	c2.cookie = &tampered
	wantStatus(t, c2.do(t, http.MethodGet, "/admin", nil), http.StatusUnauthorized)

	// A new password ends existing sessions.
	ts.addUser(t, "admin", allAuthorities...)
	wantStatus(t, c.do(t, http.MethodGet, "/admin", nil), http.StatusUnauthorized)
}

func TestLogout(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	c.login(t, "admin")
	wantStatus(t, c.do(t, http.MethodGet, "/logout", nil), http.StatusOK)
	rec := c.do(t, http.MethodPost, "/logout", url.Values{})
	wantStatus(t, rec, http.StatusSeeOther)
	wantLocation(t, rec, "/login?logout")
	if c.cookie != nil {
		t.Error("logout did not clear the session cookie")
	}
}

func TestBasicAuth(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	c.header.Set("Authorization", "Basic "+basic("admin", password))
	rec := c.do(t, http.MethodGet, "/admin", nil)
	wantStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), "<strong>1</strong> link in total") {
		t.Error("list does not show the count")
	}
	// Basic requests have no session and so no CSRF token.
	wantStatus(t, c.do(t, http.MethodDelete, "/admin/abc123", nil), http.StatusSeeOther)
	if _, ok := ts.store.links["abc123"]; ok {
		t.Error("link was not deleted")
	}
}

func TestMissingAuthority(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	ts.addUser(t, "user", "ROLE_USER", auth.Update, auth.Delete)
	ts.addUser(t, "viewer", "ROLE_USER", auth.RoleAdmin)

	c := ts.client()
	c.login(t, "user")
	wantStatus(t, c.do(t, http.MethodGet, "/admin", nil), http.StatusForbidden)

	c = ts.client()
	token := c.login(t, "viewer")
	rec := c.do(t, http.MethodGet, "/admin", nil)
	wantStatus(t, rec, http.StatusOK)
	if body := rec.Body.String(); strings.Contains(body, "delete-btn") || strings.Contains(body, "Save changes") {
		t.Error("the list offers buttons the user may not use")
	}
	form := url.Values{"_csrf": {token}, "id": {"abc123"}, "payee": {"1"}, "amount": {"2"}}
	wantStatus(t, c.do(t, http.MethodPost, "/admin/abc123", form), http.StatusForbidden)
	c.header.Set(csrfHeader, token)
	wantStatus(t, c.do(t, http.MethodDelete, "/admin/abc123", nil), http.StatusForbidden)
	if sw := ts.store.links["abc123"]; sw.Payee != "1230000000" {
		t.Errorf("link changed to %+v", sw)
	}
}

func TestCSRF(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	token := c.login(t, "admin")
	if token == "" {
		t.Fatal("admin page has no CSRF token")
	}
	other := ts.client()
	otherToken := other.login(t, "admin")

	edit := url.Values{"id": {"abc123"}, "payee": {"999"}, "amount": {"2"}}
	wantStatus(t, c.do(t, http.MethodPost, "/admin/abc123", edit), http.StatusForbidden)
	edit.Set("_csrf", otherToken)
	wantStatus(t, c.do(t, http.MethodPost, "/admin/abc123", edit), http.StatusForbidden)
	wantStatus(t, c.do(t, http.MethodDelete, "/admin/abc123", nil), http.StatusForbidden)

	// Even the right token doesn't help a cross-site request.
	edit.Set("_csrf", token)
	c.header.Set("Sec-Fetch-Site", "cross-site")
	wantStatus(t, c.do(t, http.MethodPost, "/admin/abc123", edit), http.StatusForbidden)
	evil := ts.client()
	evil.header.Set("Sec-Fetch-Site", "cross-site")
	rec := evil.do(t, http.MethodPost, "/login", url.Values{"username": {"admin"}, "password": {password}})
	wantStatus(t, rec, http.StatusForbidden)
	if evil.cookie != nil {
		t.Error("cross-site login set a session cookie")
	}

	if sw := ts.store.links["abc123"]; sw.Payee != "1230000000" {
		t.Errorf("link changed to %+v", sw)
	}

	c.header.Set("Sec-Fetch-Site", "same-origin")
	wantStatus(t, c.do(t, http.MethodPost, "/admin/abc123", edit), http.StatusSeeOther)
}

func TestEdit(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1, Description: ptr("old")})
	ts.addUser(t, "admin", allAuthorities...)
	logged := ts.captureLogs(t)
	c := ts.client()
	token := c.login(t, "admin")

	rec := c.do(t, http.MethodPost, "/admin/abc123", url.Values{
		"_csrf": {token}, "originalId": {"abc123"}, "id": {" abc123 "},
		"payee": {"0700000000"}, "amount": {"250"}, "description": {""},
	})
	wantStatus(t, rec, http.StatusSeeOther)
	wantLocation(t, rec, "/admin")
	sw := ts.store.links["abc123"]
	if sw.Payee != "0700000000" || sw.Amount != 250 || sw.Description != nil {
		t.Errorf("link = %+v, want payee 0700000000, amount 250 and no description", sw)
	}
	wantLogged(t, logged("updated link"), map[string]string{
		"id": `"abc123"`, "user": `"admin"`,
		"changed": `{"payee":{"from":"1230000000","to":"0700000000"},"amount":{"from":1,"to":250},"description":{"from":"old","to":""}}`,
	})
}

func TestRename(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	ts.addUser(t, "admin", allAuthorities...)
	logged := ts.captureLogs(t)
	c := ts.client()
	token := c.login(t, "admin")

	rec := c.do(t, http.MethodPost, "/admin/abc123", url.Values{
		"_csrf": {token}, "originalId": {"abc123"}, "id": {"fika"},
		"payee": {"1230000000"}, "amount": {"1"}, "description": {"Fika"},
	})
	wantStatus(t, rec, http.StatusSeeOther)
	if _, ok := ts.store.links["abc123"]; ok {
		t.Error("the old id still exists")
	}
	if sw := ts.store.links["fika"]; sw.Description == nil || *sw.Description != "Fika" {
		t.Errorf("renamed link = %+v", sw)
	}
	wantLogged(t, logged("renamed link"), map[string]string{
		"from": `"abc123"`, "to": `"fika"`, "user": `"admin"`,
		"changed": `{"description":{"from":"","to":"Fika"}}`,
	})
}

func TestRenameCollision(t *testing.T) {
	ts := newTestServer(t,
		sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1},
		sqlc.Swosh{ID: "fika", Payee: "0700000000", Amount: 2},
	)
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	token := c.login(t, "admin")

	rec := c.do(t, http.MethodPost, "/admin/abc123", url.Values{
		"_csrf": {token}, "originalId": {"abc123"}, "id": {"fika"}, "payee": {"1230000000"}, "amount": {"1"},
	})
	wantStatus(t, rec, http.StatusConflict)
	body := rec.Body.String()
	if !strings.Contains(body, "There is already a link with id &#34;fika&#34;") {
		t.Errorf("page does not explain the collision: %s", body)
	}
	if !strings.Contains(body, `action="/admin/abc123"`) || !strings.Contains(body, `value="fika"`) {
		t.Error("the form is not shown again with the typed id")
	}
	if ts.store.links["fika"].Payee != "0700000000" || ts.store.links["abc123"].Payee != "1230000000" {
		t.Errorf("links changed: %+v", ts.store.links)
	}
}

func TestEditValidation(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	token := c.login(t, "admin")

	for _, tt := range []struct {
		name, field, value string
		code               int
	}{
		{"blank id", "id", "  ", http.StatusBadRequest},
		{"id with slash", "id", "a/b", http.StatusBadRequest},
		{"reserved id", "id", "Admin", http.StatusBadRequest},
		{"blank payee", "payee", "", http.StatusBadRequest},
		{"fractional amount", "amount", "1.5", http.StatusBadRequest},
		{"negative amount", "amount", "-1", http.StatusBadRequest},
		{"originalId from another link", "originalId", "zzz999", http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{"_csrf": {token}, "originalId": {"abc123"}, "id": {"abc123"}, "payee": {"1230000000"}, "amount": {"1"}}
			form.Set(tt.field, tt.value)
			wantStatus(t, c.do(t, http.MethodPost, "/admin/abc123", form), tt.code)
		})
	}
	form := url.Values{"_csrf": {token}, "id": {"x"}, "payee": {"1"}, "amount": {"1"}}
	wantStatus(t, c.do(t, http.MethodPost, "/admin/nope12", form), http.StatusNotFound)
	if sw := ts.store.links["abc123"]; sw.Payee != "1230000000" || sw.Amount != 1 {
		t.Errorf("link changed to %+v", sw)
	}
}

func TestDelete(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	ts.addUser(t, "admin", allAuthorities...)
	logged := ts.captureLogs(t)
	c := ts.client()
	c.header.Set(csrfHeader, c.login(t, "admin"))

	rec := c.do(t, http.MethodDelete, "/admin/abc123", nil)
	wantStatus(t, rec, http.StatusSeeOther)
	wantLocation(t, rec, "/admin")
	if _, ok := ts.store.links["abc123"]; ok {
		t.Error("link was not deleted")
	}
	wantLogged(t, logged("deleted link"), map[string]string{"id": `"abc123"`, "user": `"admin"`})
	// Deleting it again is no error, as in the Kotlin service.
	wantStatus(t, c.do(t, http.MethodDelete, "/admin/abc123", nil), http.StatusSeeOther)
}

func TestAdminPages(t *testing.T) {
	ts := newTestServer(t,
		sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1, Description: ptr(`<b>"hi"</b>`)},
		sqlc.Swosh{ID: "def456", Payee: "0700000000", Amount: 2, ExpiresOn: ptr(now)},
	)
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	c.login(t, "admin")

	rec := c.do(t, http.MethodGet, "/admin/", nil)
	wantStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	for _, want := range []string{"<strong>2</strong> links in total", "Signed in as <strong>admin</strong>", `href="/abc123/qr"`, "2026-10-22T12:00:00Z", "&lt;b&gt;&#34;hi&#34;&lt;/b&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("list is missing %s", want)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("admin page may be cached")
	}

	rec = c.do(t, http.MethodGet, "/admin/def456", nil)
	wantStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "abc123") {
		t.Error("single page shows other links")
	}
	wantStatus(t, c.do(t, http.MethodGet, "/admin/nope12", nil), http.StatusNotFound)
}

func TestIndexShowsAdminButtonWhenLoggedIn(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "admin", allAuthorities...)
	c := ts.client()
	if strings.Contains(c.do(t, http.MethodGet, "/", nil).Body.String(), `href="/admin"`) {
		t.Error("anonymous users see the Admin button")
	}
	c.login(t, "admin")
	if !strings.Contains(c.do(t, http.MethodGet, "/", nil).Body.String(), `href="/admin"`) {
		t.Error("logged-in users don't see the Admin button")
	}
}

func TestSecurityHeaders(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1})
	for _, target := range []string{"/", "/abc123", "/admin", "/login", "/nope/nope/nope"} {
		h := ts.do(t, http.MethodGet, target, "").Header()
		if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "style-src 'self';") || !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("%s: Content-Security-Policy = %q", target, csp)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s: missing nosniff or X-Frame-Options", target)
		}
	}
}

func TestAdminMetricsUseRoutePattern(t *testing.T) {
	ts := newTestServer(t)
	ts.do(t, http.MethodGet, "/admin/abc123", "")
	rec := httptest.NewRecorder()
	ts.Metrics.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/prometheus", nil))
	want := `method="GET",status="401",uri="/admin/{id}"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("metrics are missing %s", want)
	}
}

// TestAdminWithPostgres provisions a user, logs in and edits links through
// the real queries.
func TestAdminWithPostgres(t *testing.T) {
	dbURL := dbtest.URL(t)
	log := slog.New(slog.DiscardHandler)
	if err := db.Migrate(dbURL, log); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(t.Context(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)
	if err := auth.Provision(t.Context(), q, "admin", password, log); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"abc123", "fika"} {
		if _, err := q.CreateSwosh(t.Context(), sqlc.CreateSwoshParams{ID: id, Payee: "1230000000", Amount: 1}); err != nil {
			t.Fatal(err)
		}
	}

	s := newServer(Config{Store: q, QR: &fakeQR{}, Metrics: metrics.New(metrics.Labels{}), Log: log, Scheme: "https", Hostname: "test.swosh.me"})
	c := (&testServer{server: s, handler: s.routes()}).client()
	token := c.login(t, "admin")

	edit := func(id, newID string) *httptest.ResponseRecorder {
		return c.do(t, http.MethodPost, "/admin/"+id, url.Values{
			"_csrf": {token}, "originalId": {id}, "id": {newID}, "payee": {"0700000000"}, "amount": {"5"},
		})
	}
	wantStatus(t, edit("abc123", "fika"), http.StatusConflict)
	wantStatus(t, edit("abc123", "ab"), http.StatusSeeOther)
	if sw, err := q.GetSwosh(t.Context(), "ab"); err != nil || sw.Amount != 5 {
		t.Errorf("renamed link = %+v, %v", sw, err)
	}

	c.header.Set(csrfHeader, token)
	wantStatus(t, c.do(t, http.MethodDelete, "/admin/fika", nil), http.StatusSeeOther)
	rec := c.do(t, http.MethodGet, "/admin", nil)
	if !strings.Contains(rec.Body.String(), "<strong>1</strong> link in total") {
		t.Errorf("list after delete: %s", rec.Body)
	}
}
