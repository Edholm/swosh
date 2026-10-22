package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/Edholm/swosh/internal/db/sqlc"
)

func ptr[T any](v T) *T { return &v }

func TestToImportSwosh(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	created := now.Add(-24 * time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name   string
		doc    mongoSwosh
		want   sqlc.ImportSwoshParams
		wantOK bool
	}{
		{
			name:   "all fields",
			doc:    mongoSwosh{ID: "abc123", Payee: "070-000 00 00", Amount: 100, Description: ptr("Pizza"), ExpiresOn: &future, CreatedAt: created},
			want:   sqlc.ImportSwoshParams{ID: "abc123", Payee: "070-000 00 00", Amount: 100, Description: ptr("Pizza"), ExpiresOn: &future, CreatedAt: created},
			wantOK: true,
		},
		{
			name:   "never expires, no description",
			doc:    mongoSwosh{ID: "never1", Payee: "1230000000", Amount: 1, CreatedAt: created},
			want:   sqlc.ImportSwoshParams{ID: "never1", Payee: "1230000000", Amount: 1, CreatedAt: created},
			wantOK: true,
		},
		{
			name:   "blank description becomes null",
			doc:    mongoSwosh{ID: "blank1", Payee: "p", Amount: 1, Description: ptr("  "), CreatedAt: created},
			want:   sqlc.ImportSwoshParams{ID: "blank1", Payee: "p", Amount: 1, CreatedAt: created},
			wantOK: true,
		},
		{
			name:   "missing createdAt falls back to now",
			doc:    mongoSwosh{ID: "nodate", Payee: "p", Amount: 1},
			want:   sqlc.ImportSwoshParams{ID: "nodate", Payee: "p", Amount: 1, CreatedAt: now},
			wantOK: true,
		},
		{
			name:   "already expired is skipped",
			doc:    mongoSwosh{ID: "old123", Payee: "p", Amount: 1, ExpiresOn: ptr(now.Add(-time.Second)), CreatedAt: created},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := toImportSwosh(tt.doc, now)
			if err != nil {
				t.Fatalf("toImportSwosh: %v", err)
			}
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toImportSwosh =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestToImportSwoshRejectsMissingID(t *testing.T) {
	if _, _, err := toImportSwosh(mongoSwosh{Payee: "p", Amount: 1}, time.Now()); err == nil {
		t.Error("toImportSwosh without _id: got nil error")
	}
}

func TestToCreateUser(t *testing.T) {
	const hash = "{bcrypt}$2a$10$abcdefghijklmnopqrstuv"
	tests := []struct {
		name string
		doc  mongoUser
		want sqlc.CreateUserParams
	}{
		{
			name: "admin",
			doc: mongoUser{
				Username: "admin", Password: hash, IsEnabled: ptr(true),
				Authorities: []mongoAuthority{{"ROLE_USER"}, {"ROLE_ADMIN"}, {"DELETE"}, {"UPDATE"}},
			},
			want: sqlc.CreateUserParams{
				Username: "admin", Password: hash, Enabled: true,
				Authorities: []string{"ROLE_USER", "ROLE_ADMIN", "DELETE", "UPDATE"},
			},
		},
		{
			name: "disabled",
			doc:  mongoUser{Username: "old", Password: hash, IsEnabled: ptr(false), Authorities: []mongoAuthority{{"USER"}}},
			want: sqlc.CreateUserParams{Username: "old", Password: hash, Enabled: false, Authorities: []string{"USER"}},
		},
		{
			name: "missing isEnabled and authorities",
			doc:  mongoUser{Username: "bare", Password: hash},
			want: sqlc.CreateUserParams{Username: "bare", Password: hash, Enabled: true, Authorities: []string{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toCreateUser(tt.doc)
			if err != nil {
				t.Fatalf("toCreateUser: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toCreateUser =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestToCreateUserRejectsIncompleteUser(t *testing.T) {
	for _, doc := range []mongoUser{{Username: "nopass"}, {Password: "{bcrypt}x"}} {
		if _, err := toCreateUser(doc); err == nil {
			t.Errorf("toCreateUser(%+v): got nil error", doc)
		}
	}
}
