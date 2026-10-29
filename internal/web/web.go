// Package web serves swosh's pages and API: link creation, the preview page,
// the redirect into Swish, the QR code, and the admin pages behind a login.
package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Edholm/swosh/internal/auth"
	"github.com/Edholm/swosh/internal/db/sqlc"
	"github.com/Edholm/swosh/internal/metrics"
	"github.com/Edholm/swosh/internal/swish"
)

var (
	//go:embed static
	staticFiles embed.FS
	//go:embed templates/*.html
	templateFiles embed.FS
)

const (
	idLength   = 6
	idAlphabet = "0123456789abcdefghijklmnopqrstuv"
	// createAttempts bounds retries when a random id is already taken.
	createAttempts = 5
	maxBodySize    = 64 << 10
)

// Store is the part of the database the site uses.
type Store interface {
	GetSwosh(ctx context.Context, id string) (sqlc.Swosh, error)
	CreateSwosh(ctx context.Context, arg sqlc.CreateSwoshParams) (sqlc.Swosh, error)
	ListSwoshes(ctx context.Context) ([]sqlc.Swosh, error)
	UpdateSwosh(ctx context.Context, arg sqlc.UpdateSwoshParams) (int64, error)
	DeleteSwosh(ctx context.Context, id string) (int64, error)
	GetUserByUsername(ctx context.Context, username string) (sqlc.User, error)
}

// QRCoder makes QR codes that the Swish app can scan.
type QRCoder interface {
	PNG(ctx context.Context, p swish.Payment) ([]byte, error)
}

// Config holds what New needs.
type Config struct {
	Store   Store
	QR      QRCoder
	Metrics *metrics.Metrics
	Log     *slog.Logger
	// Scheme and Hostname make up the public URL of links, e.g. https://swosh.me/abc123.
	Scheme   string
	Hostname string
	// SessionKey signs login sessions. When empty, a random key is used.
	SessionKey []byte
}

type server struct {
	Config
	tmpl        *template.Template
	now         func() time.Time
	newID       func() string
	signer      *auth.Signer
	crossOrigin *http.CrossOriginProtection
}

// New returns the handler for the public site.
func New(c Config) http.Handler {
	return newServer(c).routes()
}

func newServer(c Config) *server {
	key := c.SessionKey
	if len(key) == 0 {
		key = auth.RandomKey()
	}
	return &server{
		Config:      c,
		tmpl:        template.Must(template.ParseFS(templateFiles, "templates/*.html")),
		now:         time.Now,
		newID:       newID,
		signer:      auth.NewSigner(key),
		crossOrigin: http.NewCrossOriginProtection(),
	}
}

func (s *server) routes() http.Handler {
	// The admin pages get their own mux: /admin/{id} and /{id}/qr would
	// conflict in one.
	admin := http.NewServeMux()
	admin.Handle("GET /admin", s.admin("", s.adminList))
	admin.Handle("GET /admin/{$}", s.admin("", s.adminList))
	admin.Handle("GET /admin/{id}", s.admin("", s.adminSingle))
	admin.Handle("POST /admin/{id}", s.admin(auth.Update, s.adminUpdate))
	admin.Handle("DELETE /admin/{id}", s.admin(auth.Delete, s.adminDelete))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /logout", s.logoutForm)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("POST /api/create", s.create)
	mux.HandleFunc("GET /{id}", s.preview)
	mux.HandleFunc("GET /{id}/redir", s.redirect)
	mux.HandleFunc("GET /{id}/qr", s.qr)

	// Each static file gets its own route: a /js/ prefix route would clash
	// with /{id}/qr.
	static, _ := fs.Sub(staticFiles, "static")
	files := http.FileServerFS(static)
	_ = fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			mux.Handle("GET /"+p, files)
		}
		return err
	})
	return securityHeaders(s.instrument(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
			admin.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})))
}

// contentSecurityPolicy is the Kotlin service's policy, minus jQuery, which
// the admin page no longer needs, and the unused getbootstrap.com, and with
// rules added for forms, framing and <base>. Font Awesome's script injects a
// <style> element, hence 'unsafe-inline' for styles.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline' https://maxcdn.bootstrapcdn.com; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// securityHeaders adds the headers Spring Security sent by default, plus
// the Content-Security-Policy, to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// baseURL is the public address of the site, without a trailing slash.
func (s *server) baseURL() string {
	return s.Scheme + "://" + s.Hostname
}

type indexPage struct {
	// LoggedIn shows the Admin button.
	LoggedIn bool
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "index.html", indexPage{LoggedIn: s.currentUser(r) != nil})
}

type previewData struct {
	BaseURL     string
	ID          string
	Payee       string
	Amount      int32
	Description string
	// ExpiresOn is empty for links that never expire.
	ExpiresOn string
	SwishURI  template.URL
}

func (s *server) preview(w http.ResponseWriter, r *http.Request) {
	sw, ok := s.lookup(w, r)
	if !ok {
		return
	}
	data := previewData{
		BaseURL: s.baseURL(),
		ID:      sw.ID,
		Payee:   sw.Payee,
		Amount:  sw.Amount,
		// URI escapes everything a browser could misread, and the scheme is fixed.
		SwishURI: template.URL(payment(sw).URI()),
	}
	if sw.Description != nil {
		data.Description = *sw.Description
	}
	if sw.ExpiresOn != nil {
		data.ExpiresOn = sw.ExpiresOn.UTC().Format(time.RFC3339)
	}
	s.render(w, r, "preview.html", data)
}

func (s *server) redirect(w http.ResponseWriter, r *http.Request) {
	sw, ok := s.lookup(w, r)
	if !ok {
		return
	}
	// http.Redirect would add an HTML body; the Kotlin service sent none.
	w.Header().Set("Location", payment(sw).URI())
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (s *server) qr(w http.ResponseWriter, r *http.Request) {
	sw, ok := s.lookup(w, r)
	if !ok {
		return
	}
	png, err := s.QR.PNG(r.Context(), payment(sw))
	if err != nil {
		s.Log.ErrorContext(r.Context(), "fetch QR code", "id", sw.ID, "err", err)
		http.Error(w, "Could not get a QR code from Swish", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

// lookup finds the link named in the path. For an unknown or expired link it
// shows the not found page, and on a database error it answers 500. It
// reports whether the caller should go on.
func (s *server) lookup(w http.ResponseWriter, r *http.Request) (sqlc.Swosh, bool) {
	id := r.PathValue("id")
	sw, err := s.Store.GetSwosh(r.Context(), id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		s.Log.ErrorContext(r.Context(), "get link", "id", id, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return sqlc.Swosh{}, false
	case sw.ExpiresOn != nil && !sw.ExpiresOn.After(s.now()):
		// Expired, but the expiry job has not deleted it yet.
	default:
		return sw, true
	}
	s.renderStatus(w, r, http.StatusNotFound, "notfound.html", nil)
	return sqlc.Swosh{}, false
}

func payment(sw sqlc.Swosh) swish.Payment {
	p := swish.Payment{Payee: sw.Payee, Amount: sw.Amount}
	if sw.Description != nil {
		p.Message = *sw.Description
	}
	return p
}

type createResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type errorResponse struct {
	Error  bool   `json:"error"`
	Reason string `json:"reason"`
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(&req); err != nil {
		s.createFailed(w, http.StatusBadRequest, invalidInputReason)
		return
	}
	n, err := req.validate(s.now())
	if err != nil {
		s.createFailed(w, http.StatusBadRequest, err.Error())
		return
	}
	sw, err := s.insert(r.Context(), n)
	if err != nil {
		s.Log.ErrorContext(r.Context(), "create link", "err", err)
		s.createFailed(w, http.StatusInternalServerError, "Unable to generate Swosh!")
		return
	}
	s.Metrics.CreateSuccess.Inc()
	s.Log.InfoContext(r.Context(), "created link", "id", sw.ID)
	writeJSON(w, http.StatusOK, createResponse{ID: sw.ID, URL: s.baseURL() + "/" + sw.ID})
}

// insert stores n under a new random id, retrying if the id is taken.
func (s *server) insert(ctx context.Context, n newSwosh) (sqlc.Swosh, error) {
	var err error
	for range createAttempts {
		var sw sqlc.Swosh
		sw, err = s.Store.CreateSwosh(ctx, sqlc.CreateSwoshParams{
			ID:          s.newID(),
			Payee:       n.Payee,
			Amount:      n.Amount,
			Description: n.Description,
			ExpiresOn:   n.ExpiresOn,
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" { // unique_violation
			return sw, err
		}
	}
	return sqlc.Swosh{}, err
}

func (s *server) createFailed(w http.ResponseWriter, code int, reason string) {
	s.Metrics.CreateFailed.Inc()
	writeJSON(w, code, errorResponse{Error: true, Reason: reason})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.Log.ErrorContext(r.Context(), what, "err", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (s *server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	s.renderStatus(w, r, http.StatusOK, name, data)
}

func (s *server) renderStatus(w http.ResponseWriter, r *http.Request, code int, name string, data any) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		s.Log.ErrorContext(r.Context(), "render template", "template", name, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = buf.WriteTo(w)
}

// newID returns a random link id like the Kotlin service made: six base-32
// characters.
func newID() string {
	b := make([]byte, idLength)
	_, _ = rand.Read(b) // never fails
	for i := range b {
		b[i] = idAlphabet[b[i]%byte(len(idAlphabet))]
	}
	return string(b)
}

// instrument records request durations, labelled by route pattern the way
// Micrometer labels them by URI template.
func (s *server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// The mux sets r.Pattern on the request it was given.
		uri := "NOT_FOUND"
		if r.Pattern != "" {
			_, uri, _ = strings.Cut(r.Pattern, " ")
			uri = strings.TrimSuffix(uri, "{$}")
		}
		s.Metrics.HTTPRequests.
			WithLabelValues(r.Method, uri, strconv.Itoa(rec.status)).
			Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
