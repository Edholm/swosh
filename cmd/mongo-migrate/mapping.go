package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Edholm/swosh/internal/db/sqlc"
)

// mongoSwosh is a document in the "swosh" collection, as Spring Data wrote
// pub.edholm.db.Swosh. Fields that were null are missing from the document.
type mongoSwosh struct {
	ID          string     `bson:"_id"`
	Payee       string     `bson:"payee"`
	Amount      int32      `bson:"amount"`
	Description *string    `bson:"description"`
	ExpiresOn   *time.Time `bson:"expiresOn"`
	CreatedAt   time.Time  `bson:"createdAt"`
}

// mongoUser is a document in the "user" collection, as Spring Data wrote
// pub.edholm.db.User. Its _id is a UUID in the legacy Java byte order; it is
// not referenced anywhere, so it is not read and Postgres assigns a new one.
type mongoUser struct {
	Username    string           `bson:"username"`
	Password    string           `bson:"password"`
	IsEnabled   *bool            `bson:"isEnabled"`
	Authorities []mongoAuthority `bson:"authorities"`
}

// mongoAuthority is a Spring Security SimpleGrantedAuthority.
type mongoAuthority struct {
	Role string `bson:"role"`
}

// toImportSwosh converts a link. ok is false for links that expired before
// now and should not be imported.
func toImportSwosh(doc mongoSwosh, now time.Time) (params sqlc.ImportSwoshParams, ok bool, err error) {
	if doc.ID == "" {
		return params, false, errors.New("swosh document without _id")
	}
	if doc.ExpiresOn != nil && doc.ExpiresOn.Before(now) {
		return params, false, nil
	}

	// The API stored a blank message as null, but the admin form saved "".
	description := doc.Description
	if description != nil && strings.TrimSpace(*description) == "" {
		description = nil
	}
	// Spring's auditing always set createdAt; fall back just in case.
	createdAt := doc.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	return sqlc.ImportSwoshParams{
		ID:          doc.ID,
		Payee:       doc.Payee,
		Amount:      doc.Amount,
		Description: description,
		ExpiresOn:   doc.ExpiresOn,
		CreatedAt:   createdAt,
	}, true, nil
}

// toCreateUser converts a user, keeping the password hash as it is.
func toCreateUser(doc mongoUser) (sqlc.CreateUserParams, error) {
	if doc.Username == "" || doc.Password == "" {
		return sqlc.CreateUserParams{}, fmt.Errorf("user %q has no username or password", doc.Username)
	}

	enabled := doc.IsEnabled == nil || *doc.IsEnabled
	// Never nil: pgx would send NULL into a NOT NULL column.
	authorities := make([]string, 0, len(doc.Authorities))
	for _, a := range doc.Authorities {
		if a.Role != "" {
			authorities = append(authorities, a.Role)
		}
	}

	return sqlc.CreateUserParams{
		Username:    doc.Username,
		Password:    doc.Password,
		Enabled:     enabled,
		Authorities: authorities,
	}, nil
}
