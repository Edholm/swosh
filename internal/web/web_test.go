package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
	"github.com/Edholm/swosh/internal/metrics"
	"github.com/Edholm/swosh/internal/swish"
)

var now = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// fakeStore keeps links in memory. CreateSwosh fails with a unique violation
// for ids that are taken, like Postgres.
type fakeStore struct {
	links   map[string]sqlc.Swosh
	users   map[string]sqlc.User
	created []sqlc.CreateSwoshParams
	err     error
}

func (f *fakeStore) GetSwosh(_ context.Context, id string) (sqlc.Swosh, error) {
	if f.err != nil {
		return sqlc.Swosh{}, f.err
	}
	sw, ok := f.links[id]
	if !ok {
		return sqlc.Swosh{}, pgx.ErrNoRows
	}
	return sw, nil
}

func (f *fakeStore) CreateSwosh(_ context.Context, arg sqlc.CreateSwoshParams) (sqlc.Swosh, error) {
	if f.err != nil {
		return sqlc.Swosh{}, f.err
	}
	f.created = append(f.created, arg)
	if _, ok := f.links[arg.ID]; ok {
		return sqlc.Swosh{}, &pgconn.PgError{Code: "23505"}
	}
	sw := sqlc.Swosh{
		ID: arg.ID, Payee: arg.Payee, Amount: arg.Amount,
		Description: arg.Description, ExpiresOn: arg.ExpiresOn, CreatedAt: now,
	}
	f.links[arg.ID] = sw
	return sw, nil
}

type fakeQR struct {
	got swish.Payment
	err error
}

func (f *fakeQR) PNG(_ context.Context, p swish.Payment) ([]byte, error) {
	f.got = p
	return []byte("png"), f.err
}

type testServer struct {
	*server
	store   *fakeStore
	qr      *fakeQR
	handler http.Handler
}

func newTestServer(t *testing.T, links ...sqlc.Swosh) *testServer {
	t.Helper()
	store := &fakeStore{links: map[string]sqlc.Swosh{}, users: map[string]sqlc.User{}}
	for _, l := range links {
		store.links[l.ID] = l
	}
	qr := &fakeQR{}
	s := newServer(Config{
		Store:    store,
		QR:       qr,
		Metrics:  metrics.New(metrics.Labels{}),
		Log:      slog.New(slog.DiscardHandler),
		Scheme:   "https",
		Hostname: "test.swosh.me",
	})
	s.now = func() time.Time { return now }
	return &testServer{server: s, store: store, qr: qr, handler: s.routes()}
}

func (ts *testServer) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, target, r)
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

func ptr[T any](v T) *T { return &v }

func TestNotFoundWhenLinkIsMissing(t *testing.T) {
	expired := sqlc.Swosh{ID: "old123", Payee: "1230000000", Amount: 1, ExpiresOn: ptr(now.Add(-time.Second))}
	ts := newTestServer(t, expired)
	for _, target := range []string{"/asdfg", "/asdfg/redir", "/asdfg/qr", "/old123", "/old123/redir"} {
		t.Run(target, func(t *testing.T) {
			rec := ts.do(t, http.MethodGet, target, "")
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if loc := rec.Header().Get("Location"); loc != "" {
				t.Errorf("Location = %q, want none", loc)
			}
			if !strings.Contains(rec.Body.String(), "Link not found") {
				t.Errorf("body = %q, want the not found page", rec.Body)
			}
		})
	}
}

func TestDatabaseErrorIsNotARedirect(t *testing.T) {
	ts := newTestServer(t)
	ts.store.err = errors.New("connection refused")
	if rec := ts.do(t, http.MethodGet, "/asdfg", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRedirectToSwish(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "asdf123", Payee: "070-000 00 00", Amount: 100, Description: ptr("msg")})
	rec := ts.do(t, http.MethodGet, "/asdf123/redir", "")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	want := swish.Payment{Payee: "070-000 00 00", Amount: 100, Message: "msg"}.URI()
	if loc := rec.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body)
	}
}

func TestPreview(t *testing.T) {
	ts := newTestServer(t,
		sqlc.Swosh{ID: "abc123", Payee: "070-000 00 00", Amount: 100, Description: ptr("<b>pizza</b>"), ExpiresOn: ptr(now.Add(time.Hour))},
		sqlc.Swosh{ID: "forevr", Payee: "1230000000", Amount: 5},
	)

	rec := ts.do(t, http.MethodGet, "/abc123", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<meta property="og:title" content="Swish 100kr to 070-000 00 00"/>`,
		`<meta property="og:image" content="https://test.swosh.me/abc123/qr">`,
		`<dd>&lt;b&gt;pizza&lt;/b&gt;</dd>`,
		`<time datetime="2026-10-22T13:00:00Z">`,
		`href="swish://payment?data=%7B%22version%22%3A1%2C`,
		`src="/abc123/qr"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview is missing %s", want)
		}
	}

	body = ts.do(t, http.MethodGet, "/forevr", "").Body.String()
	for _, want := range []string{`content="No message given..."`, `<dd>Never</dd>`} {
		if !strings.Contains(body, want) {
			t.Errorf("preview of a link without message and expiry is missing %s", want)
		}
	}
	if strings.Contains(body, "<dt>Message</dt>") {
		t.Error("preview of a link without message shows a description")
	}
}

func TestQR(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "abc123", Payee: "070-000 00 00", Amount: 100})
	rec := ts.do(t, http.MethodGet, "/abc123/qr", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Body.String() != "png" {
		t.Errorf("got %d %q %q, want a PNG", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	if want := (swish.Payment{Payee: "070-000 00 00", Amount: 100}); ts.qr.got != want {
		t.Errorf("QR for %+v, want %+v", ts.qr.got, want)
	}

	ts.qr.err = errors.New("swish is down")
	if rec := ts.do(t, http.MethodGet, "/abc123/qr", ""); rec.Code != http.StatusBadGateway {
		t.Errorf("status with Swish down = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestIndexAndStaticFiles(t *testing.T) {
	ts := newTestServer(t)
	for _, target := range []string{"/", "/css/swosh.css", "/js/swosh.js", "/js/preview.js", "/js/time.js", "/img/icon.svg", "/img/icons.svg", "/img/og.png"} {
		if rec := ts.do(t, http.MethodGet, target, ""); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", target, rec.Code)
		}
	}
	index := ts.do(t, http.MethodGet, "/", "").Body.String()
	for _, want := range []string{`<meta property="og:image" content="https://test.swosh.me/img/og.png">`, `data-max="150000"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index is missing %s", want)
		}
	}
}

func TestCreateErrors(t *testing.T) {
	long := strings.Repeat("a", 51)
	tests := []struct {
		name, body, reason string
	}{
		{"invalid body", `{"version":1,"payee":{"value":"herp"},"amount":{"value":1},"message":{"value":"derp"}}`, "Invalid input format!"},
		{"not JSON", `phone=0700000000`, "Invalid input format!"},
		{"amount not a number", `{"phone":"0700000000","amount":"lots"}`, "Invalid input format!"},
		{"amount over the Swish limit", `{"phone":"0700000000","amount":150001}`, "Maximum allowed amount is 150000. Got 150001"},
		{"amount over int32", `{"phone":"0700000000","amount":2147483648}`, "Maximum allowed amount is 150000. Got 2147483648"},
		{"amount over int64", `{"phone":"0700000000","amount":"` + strings.Repeat("9", 30) + `"}`, "Invalid input format!"},
		// The body is cut off at maxBodySize, so the decoder never sees all of it.
		{"huge amount", `{"phone":"0700000000","amount":"` + strings.Repeat("9", 1<<20) + `"}`, "Invalid input format!"},
		{"missing amount", `{"phone":"0700000000","message":"msg","expireAfterSeconds":100}`, "Missing input parameters. 'phone' and 'amount' is required"},
		{"empty amount from the form", `{"phone":"0700000000","amount":""}`, "Missing input parameters. 'phone' and 'amount' is required"},
		{"missing phone", `{"amount":100,"message":"msg","expireAfterSeconds":100}`, "Missing input parameters. 'phone' and 'amount' is required"},
		{"blank phone", `{"phone":"          ","amount":100,"message":"msg"}`, "Missing input parameters. 'phone' and 'amount' is required"},
		{"zero amount", `{"phone":"0700000000","amount":0,"message":"msg"}`, "Minimum allowed amount is 1. Got 0"},
		{"negative amount", `{"phone":"0700000000","amount":-1,"message":"msg"}`, "Minimum allowed amount is 1. Got -1"},
		{"too long description", `{"phone":"0700000000","amount":100,"message":"` + long + `"}`, "Description is too long. Max 50 chars. Got 51"},
		{"invalid phone number", `{"phone":"070000000a","amount":100,"message":"msg"}`, "'070000000a' is not a mobile number"},
		{"unparsable phone number", `{"phone":"x","amount":100}`, "'x' is not a valid phone number"},
		{"landline", `{"phone":"08-123 456","amount":100}`, "'08-123 456' is not a mobile number"},
		{"foreign mobile", `{"phone":"+4791234567","amount":100}`, "'+4791234567' is not a mobile number"},
		{"negative expiry", `{"phone":"0700000000","amount":100,"expireAfterSeconds":-1}`, "'expireAfterSeconds' must be between 0 and 3153600000. Got -1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.do(t, http.MethodPost, "/api/create", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			var got errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode %q: %v", rec.Body, err)
			}
			if want := (errorResponse{Error: true, Reason: tt.reason}); got != want {
				t.Errorf("response = %+v, want %+v", got, want)
			}
			if len(ts.store.created) != 0 {
				t.Errorf("stored %+v, want nothing", ts.store.created)
			}
			if n := testutil.ToFloat64(ts.Metrics.CreateFailed); n != 1 {
				t.Errorf("create_failed_total = %v, want 1", n)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name string
		body string
		want sqlc.CreateSwoshParams
	}{
		{
			name: "max length description",
			body: `{"phone":"0700000000","amount":100,"message":"` + strings.Repeat("b", 50) + `","expireAfterSeconds":100}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, Description: ptr(strings.Repeat("b", 50)), ExpiresOn: ptr(now.Add(100 * time.Second))},
		},
		{
			name: "multi-byte description counted in characters",
			body: `{"phone":"0700000000","amount":100,"message":"` + strings.Repeat("ö", 50) + `"}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, Description: ptr(strings.Repeat("ö", 50)), ExpiresOn: ptr(now.Add(defaultExpiry))},
		},
		{
			name: "largest amount Swish allows",
			body: `{"phone":"0700000000","amount":150000}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 150000, ExpiresOn: ptr(now.Add(defaultExpiry))},
		},
		{
			name: "missing message",
			body: `{"phone":"0700000000","amount":100,"expireAfterSeconds":100}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, ExpiresOn: ptr(now.Add(100 * time.Second))},
		},
		{
			name: "blank message",
			body: `{"phone":"0700000000","amount":100,"message":"   "}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, ExpiresOn: ptr(now.Add(defaultExpiry))},
		},
		{
			name: "missing expiry gets the default",
			body: `{"phone":"0700000000","amount":100,"message":"msg"}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, Description: ptr("msg"), ExpiresOn: ptr(now.Add(defaultExpiry))},
		},
		{
			name: "null expiry never expires",
			body: `{"phone":"0700000000","amount":100,"message":"msg","expireAfterSeconds":null}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, Description: ptr("msg")},
		},
		{
			name: "zero expiry never expires",
			body: `{"phone":" 070-000 00 00 ","amount":100,"message":"msg","expireAfterSeconds":0}`,
			want: sqlc.CreateSwoshParams{Payee: "070-000 00 00", Amount: 100, Description: ptr("msg")},
		},
		{
			// The web form sends every value as a string, and "" for "Never".
			name: "form values as strings",
			body: `{"phone":"1230000000","amount":"25","message":"","expireAfterSeconds":""}`,
			want: sqlc.CreateSwoshParams{Payee: "1230000000", Amount: 25},
		},
		{
			name: "form values with expiry",
			body: `{"phone":"0701234567","amount":"25","message":"","expireAfterSeconds":"3600"}`,
			want: sqlc.CreateSwoshParams{Payee: "070-123 45 67", Amount: 25, ExpiresOn: ptr(now.Add(time.Hour))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.newID = func() string { return "edaeda" }
			rec := ts.do(t, http.MethodPost, "/api/create", tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (%s), want 200", rec.Code, rec.Body)
			}
			var got createResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode %q: %v", rec.Body, err)
			}
			if want := (createResponse{ID: "edaeda", URL: "https://test.swosh.me/edaeda"}); got != want {
				t.Errorf("response = %+v, want %+v", got, want)
			}
			if len(ts.store.created) != 1 {
				t.Fatalf("stored %d links, want 1", len(ts.store.created))
			}
			tt.want.ID = "edaeda"
			assertParams(t, ts.store.created[0], tt.want)
			if n := testutil.ToFloat64(ts.Metrics.CreateSuccess); n != 1 {
				t.Errorf("create_success_total = %v, want 1", n)
			}
		})
	}
}

func TestCreateAcceptsValidPhoneNumbers(t *testing.T) {
	for _, phone := range []string{"072-000 00 00", "0730 00 00 00", "+46760000000", "0790000000", "070 000 00 00", "1230000000"} {
		t.Run(phone, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.do(t, http.MethodPost, "/api/create", `{"phone":"`+phone+`","amount":100,"message":"msg","expireAfterSeconds":100}`)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d (%s), want 200", rec.Code, rec.Body)
			}
		})
	}
}

func TestCreateRetriesTakenIDs(t *testing.T) {
	ts := newTestServer(t, sqlc.Swosh{ID: "taken1"})
	ids := []string{"taken1", "free11"}
	ts.newID = func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	rec := ts.do(t, http.MethodPost, "/api/create", `{"phone":"0700000000","amount":1}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"free11"`) {
		t.Errorf("got %d %s, want 200 with id free11", rec.Code, rec.Body)
	}
}

func TestCreateDatabaseError(t *testing.T) {
	ts := newTestServer(t)
	ts.store.err = errors.New("connection refused")
	rec := ts.do(t, http.MethodPost, "/api/create", `{"phone":"0700000000","amount":1}`)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "Unable to generate Swosh!") {
		t.Errorf("got %d %s, want 500 with a reason", rec.Code, rec.Body)
	}
}

func TestRequestMetrics(t *testing.T) {
	ts := newTestServer(t,
		sqlc.Swosh{ID: "abc123", Payee: "1230000000", Amount: 1},
		sqlc.Swosh{ID: "def456", Payee: "1230000000", Amount: 2},
	)
	ts.do(t, http.MethodGet, "/abc123/redir", "")
	ts.do(t, http.MethodGet, "/def456/redir", "")
	if n := testutil.CollectAndCount(ts.Metrics.HTTPRequests); n != 1 {
		t.Errorf("got %d series, want 1 (both requests share route and status)", n)
	}
	rec := httptest.NewRecorder()
	ts.Metrics.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/prometheus", nil))
	want := `http_server_requests_seconds_count{app="",env="",host="",method="GET",status="307",uri="/{id}/redir"} 2`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("metrics are missing %s", want)
	}
}

// TestCreateAndPreviewWithPostgres runs a link through the real queries.
func TestCreateAndPreviewWithPostgres(t *testing.T) {
	url := dbtest.URL(t)
	if err := db.Migrate(url, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	h := New(Config{
		Store:    sqlc.New(pool),
		QR:       &fakeQR{},
		Metrics:  metrics.New(metrics.Labels{}),
		Log:      slog.New(slog.DiscardHandler),
		Scheme:   "https",
		Hostname: "test.swosh.me",
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", strings.NewReader(`{"phone":"0700000000","amount":100,"message":"msg"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+created.ID, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "070-000 00 00") {
		t.Errorf("preview: %d, want 200 showing the payee", rec.Code)
	}
}

func assertParams(t *testing.T, got, want sqlc.CreateSwoshParams) {
	t.Helper()
	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	tm := func(p *time.Time) string {
		if p == nil {
			return "<nil>"
		}
		return p.String()
	}
	if got.ID != want.ID || got.Payee != want.Payee || got.Amount != want.Amount ||
		str(got.Description) != str(want.Description) || tm(got.ExpiresOn) != tm(want.ExpiresOn) {
		t.Errorf("stored {%s %q %d %q %s}, want {%s %q %d %q %s}",
			got.ID, got.Payee, got.Amount, str(got.Description), tm(got.ExpiresOn),
			want.ID, want.Payee, want.Amount, str(want.Description), tm(want.ExpiresOn))
	}
}
