package web

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Edholm/swosh/internal/auth"
)

const (
	sessionCookie = "swosh_session"
	sessionTTL    = 12 * time.Hour
	csrfField     = "_csrf"
	csrfHeader    = "X-CSRF-Token"
	basicRealm    = `Basic realm="swosh", charset="UTF-8"`
)

// principal is the user a request is made by.
type principal struct {
	Name        string
	Authorities []string
	// session is nil when the request used HTTP Basic.
	session *auth.Session
}

func (p *principal) has(authority string) bool {
	return slices.Contains(p.Authorities, authority)
}

// csrfToken is the token forms must send back, or "" for HTTP Basic.
func (s *server) csrfToken(p *principal) string {
	if p.session == nil {
		return ""
	}
	return s.signer.CSRFToken(*p.session)
}

// sessionUser returns the user of the request's session cookie, or nil if
// there is no valid session. Errors are database errors.
func (s *server) sessionUser(r *http.Request) (*principal, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, nil
	}
	sess, err := s.signer.Decode(c.Value, s.now())
	if err != nil {
		return nil, nil
	}
	u, err := s.Store.GetUserByUsername(r.Context(), sess.Username)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	case !u.Enabled || s.signer.PasswordTag(u.Password) != sess.Password:
		return nil, nil
	}
	return &principal{Name: u.Username, Authorities: u.Authorities, session: &sess}, nil
}

// authenticate returns the user of the request's session or, failing that,
// its HTTP Basic credentials. It returns nil for anonymous requests and
// auth.ErrBadCredentials for wrong Basic credentials.
func (s *server) authenticate(r *http.Request) (*principal, error) {
	p, err := s.sessionUser(r)
	if p != nil || err != nil {
		return p, err
	}
	name, password, ok := r.BasicAuth()
	if !ok {
		return nil, nil
	}
	u, err := auth.Login(r.Context(), s.Store, name, password)
	if err != nil {
		return nil, err
	}
	return &principal{Name: u.Username, Authorities: u.Authorities}, nil
}

type adminHandlerFunc func(w http.ResponseWriter, r *http.Request, p *principal)

// admin lets requests through to h only for users with ROLE_ADMIN and, if
// it is not empty, authority.
//
// Unsafe methods are checked for CSRF twice: browsers must not mark the
// request as cross-site (Sec-Fetch-Site/Origin), and session users must send
// the CSRF token of their session. HTTP Basic requests have no session and so
// no token; browsers that cached Basic credentials are covered by the
// cross-site check, which they can't get around.
func (s *server) admin(authority string, h adminHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if err := s.crossOrigin.Check(r); err != nil {
			http.Error(w, "Cross-origin request rejected", http.StatusForbidden)
			return
		}
		p, err := s.authenticate(r)
		switch {
		case errors.Is(err, auth.ErrBadCredentials):
			s.challenge(w)
			return
		case err != nil:
			s.serverError(w, r, "authenticate", err)
			return
		case p == nil:
			s.loginRequired(w, r)
			return
		}
		if !p.has(auth.RoleAdmin) || (authority != "" && !p.has(authority)) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		if p.session != nil && !isSafeMethod(r.Method) {
			token := r.Header.Get(csrfHeader)
			if token == "" {
				token = r.PostFormValue(csrfField)
			}
			if !s.signer.CheckCSRFToken(*p.session, token) {
				http.Error(w, "Missing or invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		h(w, r, p)
	})
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// loginRequired sends browsers to the login form and asks other clients
// for HTTP Basic credentials, as Spring did with both enabled.
func (s *server) loginRequired(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	s.challenge(w)
}

func (s *server) challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", basicRealm)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

type loginPage struct {
	Next      string
	Username  string
	Error     string
	LoggedOut bool
}

func (s *server) loginForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q := r.URL.Query()
	s.render(w, r, "login.html", loginPage{Next: localPath(q.Get("next")), LoggedOut: q.Has("logout")})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := s.crossOrigin.Check(r); err != nil {
		http.Error(w, "Cross-origin request rejected", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	name, password := r.PostFormValue("username"), r.PostFormValue("password")
	next := localPath(r.PostFormValue("next"))

	u, err := auth.Login(r.Context(), s.Store, name, password)
	if errors.Is(err, auth.ErrBadCredentials) {
		slog.InfoContext(r.Context(), "failed login", slog.String("username", name))
		s.renderStatus(w, r, http.StatusUnauthorized, "login.html", loginPage{Next: next, Username: name, Error: "Bad credentials"})
		return
	}
	if err != nil {
		s.serverError(w, r, "log in", err)
		return
	}
	sess := s.signer.New(u.Username, u.Password, s.now().Add(sessionTTL))
	http.SetCookie(w, s.sessionCookie(s.signer.Encode(sess), sess.Expires))
	slog.InfoContext(r.Context(), "logged in", slog.String("username", u.Username))
	if next == "" {
		next = "/admin"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// logoutForm asks before logging out, like Spring's GET /logout page.
func (s *server) logoutForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, r, "logout.html", nil)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.crossOrigin.Check(r); err != nil {
		http.Error(w, "Cross-origin request rejected", http.StatusForbidden)
		return
	}
	http.SetCookie(w, s.sessionCookie("", time.Unix(0, 0)))
	http.Redirect(w, r, "/login?logout", http.StatusSeeOther)
}

func (s *server) sessionCookie(value string, expires time.Time) *http.Cookie {
	c := &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
	}
	if value == "" {
		c.MaxAge = -1
	}
	return c
}

// localPath returns p if it is a path on this site, and "" otherwise, so
// that ?next= can't send users elsewhere after logging in.
func localPath(p string) string {
	// Browsers drop tabs and newlines and read \ as /, so "/\t/evil.com"
	// and "/\\evil.com" would both go to evil.com.
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") ||
		strings.ContainsFunc(p, func(c rune) bool { return c < 0x20 || c == 0x7f || c == '\\' }) {
		return ""
	}
	return p
}

// currentUser is the session user for pages that only change their looks
// for logged-in users. Database errors are logged and treated as anonymous.
func (s *server) currentUser(r *http.Request) *principal {
	p, err := s.sessionUser(r)
	if err != nil {
		slog.ErrorContext(r.Context(), "look up session user", slog.Any("err", err))
	}
	return p
}
