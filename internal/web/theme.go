package web

import (
	"net/http"
	"time"
)

// theme is the colour theme picked with the switch in the footer. Without
// one, the pages follow the system through prefers-color-scheme.
type theme string

const (
	systemTheme theme = ""
	lightTheme  theme = "light"
	darkTheme   theme = "dark"
)

// themes are the themes the templates are parsed for.
var themes = []theme{systemTheme, lightTheme, darkTheme}

const (
	themeCookie    = "theme"
	themeCookieAge = 365 * 24 * time.Hour
)

// requestTheme is the theme picked in the footer, or systemTheme.
func requestTheme(r *http.Request) theme {
	if c, err := r.Cookie(themeCookie); err == nil {
		if t, ok := parseTheme(c.Value); ok {
			return t
		}
	}
	return systemTheme
}

func parseTheme(s string) (theme, bool) {
	switch t := theme(s); t {
	case lightTheme, darkTheme:
		return t, true
	}
	return "", false
}

// setTheme is the theme switch in the footer: GET /theme?t=dark remembers the
// theme in a cookie and goes back to the page it was clicked on.
func (s *server) setTheme(w http.ResponseWriter, r *http.Request) {
	if t, ok := parseTheme(r.URL.Query().Get("t")); ok {
		http.SetCookie(w, &http.Cookie{
			Name:     themeCookie,
			Value:    string(t),
			Path:     "/",
			MaxAge:   int(themeCookieAge / time.Second),
			HttpOnly: true,
			Secure:   s.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
		})
	}
	http.Redirect(w, r, backTo(r), http.StatusSeeOther)
}
