package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Edholm/swosh/internal/auth"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

// validID is what admins may rename links to. Ids end up in URL paths, so
// slashes and the like are out.
var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// reservedIDs are first path segments the site uses itself. A link with one
// of these ids could never be opened.
var reservedIDs = []string{"admin", "api", "lang", "login", "logout"}

type adminPage struct {
	User  string
	CSRF  string
	Error string
	// Count and CanDelete are used on the list page only.
	Count     int
	CanDelete bool
	Links     []adminLink
}

// adminLink is one link's edit form. The fields hold form values as typed,
// so a rejected edit can be shown again.
type adminLink struct {
	// OriginalID is the link's id in the database, and ID the one in the
	// form, which differs when renaming.
	OriginalID  string
	ID          string
	Payee       string
	Amount      string
	Description string
	ExpiresOn   string
	CreatedAt   string
	CanUpdate   bool
	CanDelete   bool
}

func (s *server) newAdminPage(p *principal) adminPage {
	return adminPage{User: p.Name, CSRF: s.csrfToken(p)}
}

func (s *server) adminLinkOf(sw sqlc.Swosh, p *principal) adminLink {
	l := adminLink{
		OriginalID: sw.ID,
		ID:         sw.ID,
		Payee:      sw.Payee,
		Amount:     strconv.Itoa(int(sw.Amount)),
		CreatedAt:  sw.CreatedAt.UTC().Format(time.RFC3339),
		CanUpdate:  p.has(auth.Update),
		CanDelete:  p.has(auth.Delete),
	}
	if sw.Description != nil {
		l.Description = *sw.Description
	}
	if sw.ExpiresOn != nil {
		l.ExpiresOn = sw.ExpiresOn.UTC().Format(time.RFC3339)
	}
	return l
}

func (s *server) adminList(w http.ResponseWriter, r *http.Request, p *principal) {
	links, err := s.Store.ListSwoshes(r.Context())
	if err != nil {
		s.serverError(w, r, "list links", err)
		return
	}
	page := s.newAdminPage(p)
	page.Count = len(links)
	page.CanDelete = p.has(auth.Delete)
	for _, sw := range links {
		page.Links = append(page.Links, s.adminLinkOf(sw, p))
	}
	s.render(w, r, "admin.html", page)
}

func (s *server) adminSingle(w http.ResponseWriter, r *http.Request, p *principal) {
	sw, ok := s.adminLookup(w, r, p)
	if !ok {
		return
	}
	page := s.newAdminPage(p)
	page.Links = []adminLink{s.adminLinkOf(sw, p)}
	s.render(w, r, "admin-single.html", page)
}

// adminLookup gets the link named in the path, or shows a 404 page.
func (s *server) adminLookup(w http.ResponseWriter, r *http.Request, p *principal) (sqlc.Swosh, bool) {
	id := r.PathValue("id")
	sw, err := s.Store.GetSwosh(r.Context(), id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		page := s.newAdminPage(p)
		page.Error = fmt.Sprintf("There is no link with id %q.", id)
		s.renderStatus(w, r, http.StatusNotFound, "admin-single.html", page)
		return sqlc.Swosh{}, false
	case err != nil:
		s.serverError(w, r, "get link", err)
		return sqlc.Swosh{}, false
	}
	return sw, true
}

// adminUpdate saves an edit and renames the link when the id changed. Unlike
// the Kotlin service, a rename onto an existing id is refused rather than
// overwriting that link.
func (s *server) adminUpdate(w http.ResponseWriter, r *http.Request, p *principal) {
	sw, ok := s.adminLookup(w, r, p)
	if !ok {
		return
	}
	if orig := r.PostFormValue("originalId"); orig != "" && orig != sw.ID {
		http.Error(w, "originalId does not match the URL", http.StatusBadRequest)
		return
	}
	form := s.adminLinkOf(sw, p)
	form.ID = strings.TrimSpace(r.PostFormValue("id"))
	form.Payee = strings.TrimSpace(r.PostFormValue("payee"))
	form.Amount = strings.TrimSpace(r.PostFormValue("amount"))
	form.Description = r.PostFormValue("description")

	failed := func(code int, msg string) {
		page := s.newAdminPage(p)
		page.Error = msg
		page.Links = []adminLink{form}
		s.renderStatus(w, r, code, "admin-single.html", page)
	}

	arg, msg := form.params()
	if msg != "" {
		failed(http.StatusBadRequest, msg)
		return
	}
	n, err := s.Store.UpdateSwosh(r.Context(), arg)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505": // unique_violation
		failed(http.StatusConflict, fmt.Sprintf("There is already a link with id %q. Pick another id.", arg.NewID))
		return
	case err != nil:
		s.serverError(w, r, "update link", err)
		return
	case n == 0:
		// Deleted since adminLookup.
		failed(http.StatusNotFound, fmt.Sprintf("There is no link with id %q.", arg.ID))
		return
	}
	changed := slog.GroupAttrs("changed", changes(sw, arg)...)
	if arg.NewID != arg.ID {
		slog.InfoContext(r.Context(), "renamed link", slog.String("from", arg.ID), slog.String("to", arg.NewID), slog.String("user", p.Name), changed)
	} else {
		slog.InfoContext(r.Context(), "updated link", slog.String("id", arg.ID), slog.String("user", p.Name), changed)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// changes returns a group of old and new values for each field an edit
// changes, so the log shows what an admin did.
func changes(sw sqlc.Swosh, arg sqlc.UpdateSwoshParams) []slog.Attr {
	var attrs []slog.Attr
	change := func(field string, from, to any) {
		attrs = append(attrs, slog.GroupAttrs(field, slog.Any("from", from), slog.Any("to", to)))
	}
	if arg.Payee != sw.Payee {
		change("payee", sw.Payee, arg.Payee)
	}
	if arg.Amount != sw.Amount {
		change("amount", sw.Amount, arg.Amount)
	}
	if from, to := deref(sw.Description), deref(arg.Description); from != to {
		change("description", from, to)
	}
	return attrs
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// params validates the form and returns a message for the admin if it is
// invalid. Only what would break the link is checked: many existing links
// break the rules the create API enforces, and admins must still be able to
// edit them.
func (l adminLink) params() (sqlc.UpdateSwoshParams, string) {
	switch {
	case l.ID == "":
		return sqlc.UpdateSwoshParams{}, "The id must not be blank."
	case !validID.MatchString(l.ID):
		return sqlc.UpdateSwoshParams{}, "The id may only contain letters, digits, - and _, and be at most 64 characters."
	case slices.Contains(reservedIDs, strings.ToLower(l.ID)):
		return sqlc.UpdateSwoshParams{}, fmt.Sprintf("%q is used by swosh itself and can't be a link id.", l.ID)
	case l.Payee == "":
		return sqlc.UpdateSwoshParams{}, "The phone number must not be blank."
	}
	amount, err := strconv.ParseInt(l.Amount, 10, 32)
	if err != nil || amount < 0 {
		return sqlc.UpdateSwoshParams{}, "The amount must be a whole number, 0 or more."
	}
	arg := sqlc.UpdateSwoshParams{ID: l.OriginalID, NewID: l.ID, Payee: l.Payee, Amount: int32(amount)}
	if strings.TrimSpace(l.Description) != "" {
		arg.Description = &l.Description
	}
	return arg, ""
}

func (s *server) adminDelete(w http.ResponseWriter, r *http.Request, p *principal) {
	id := r.PathValue("id")
	n, err := s.Store.DeleteSwosh(r.Context(), id)
	if err != nil {
		s.serverError(w, r, "delete link", err)
		return
	}
	// Like the Kotlin service, deleting a missing link is not an error.
	if n > 0 {
		slog.InfoContext(r.Context(), "deleted link", slog.String("id", id), slog.String("user", p.Name))
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// maxDeleteMany caps how many links one bulk delete may name.
const maxDeleteMany = 1000

// adminDeleteMany deletes the links whose ids are in the JSON body
// {"ids": [...]}. Missing links are skipped, like in adminDelete.
func (s *server) adminDeleteMany(w http.ResponseWriter, r *http.Request, p *principal) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Body must be JSON like {\"ids\": [\"abc123\"]}", http.StatusBadRequest)
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > maxDeleteMany {
		http.Error(w, fmt.Sprintf("Name between 1 and %d ids", maxDeleteMany), http.StatusBadRequest)
		return
	}
	deleted, err := s.Store.DeleteSwoshes(r.Context(), body.IDs)
	if err != nil {
		s.serverError(w, r, "delete links", err)
		return
	}
	if len(deleted) > 0 {
		slog.InfoContext(r.Context(), "deleted links", slog.Any("ids", deleted), slog.String("user", p.Name))
	}
	w.WriteHeader(http.StatusNoContent)
}
