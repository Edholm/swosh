package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Sessions are stateless: the cookie holds the username, an expiry time and
// a nonce, signed with HMAC-SHA256. swosh runs as a single instance with a
// handful of admins, so a sessions table and its cleanup would buy little.
// The price is that a session can't be revoked on the server; it ends when it
// expires, when the user's password changes or is disabled (both checked on
// every request), or when the key changes.

// ErrInvalidSession means a cookie was tampered with, malformed or expired.
var ErrInvalidSession = errors.New("invalid session")

// Session is a logged-in user.
type Session struct {
	Username string    `json:"u"`
	Expires  time.Time `json:"e"`
	// Nonce makes each login's CSRF token different.
	Nonce string `json:"n"`
	// Password is a keyed hash of the user's password hash, so changing the
	// password ends existing sessions.
	Password string `json:"p"`
}

// Signer signs sessions and CSRF tokens with a secret key.
type Signer struct {
	key []byte
}

// NewSigner returns a Signer that uses key.
func NewSigner(key []byte) *Signer {
	return &Signer{key: key}
}

// RandomKey returns a new 32-byte key.
func RandomKey() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails
	return b
}

// New starts a session for a user whose stored password hash is
// passwordHash.
func (s *Signer) New(username, passwordHash string, expires time.Time) Session {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	return Session{
		Username: username,
		Expires:  expires.UTC().Truncate(time.Second),
		Nonce:    b64.EncodeToString(nonce),
		Password: s.PasswordTag(passwordHash),
	}
}

// PasswordTag is what a Session stores about the password hash.
func (s *Signer) PasswordTag(passwordHash string) string {
	return b64.EncodeToString(s.mac("password", passwordHash)[:16])
}

var b64 = base64.RawURLEncoding

// Encode returns the cookie value for sess.
func (s *Signer) Encode(sess Session) string {
	payload, _ := json.Marshal(sess) // can't fail for this type
	p := b64.EncodeToString(payload)
	return p + "." + b64.EncodeToString(s.mac("session", p))
}

// Decode checks the signature and expiry of a cookie value made by Encode.
func (s *Signer) Decode(value string, now time.Time) (Session, error) {
	p, sig, ok := strings.Cut(value, ".")
	if !ok {
		return Session{}, ErrInvalidSession
	}
	got, err := b64.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac("session", p)) {
		return Session{}, ErrInvalidSession
	}
	payload, err := b64.DecodeString(p)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	var sess Session
	if err := json.Unmarshal(payload, &sess); err != nil || sess.Username == "" || sess.Nonce == "" {
		return Session{}, ErrInvalidSession
	}
	if !now.Before(sess.Expires) {
		return Session{}, ErrInvalidSession
	}
	return sess, nil
}

// CSRFToken returns the token that forms in sess must send back.
func (s *Signer) CSRFToken(sess Session) string {
	return b64.EncodeToString(s.mac("csrf", sess.Nonce))
}

// CheckCSRFToken reports whether token is the CSRF token of sess.
func (s *Signer) CheckCSRFToken(sess Session, token string) bool {
	got, err := b64.DecodeString(token)
	return err == nil && hmac.Equal(got, s.mac("csrf", sess.Nonce))
}

// mac is HMAC-SHA256 over purpose and value. The purpose keeps a signature
// made for one use from being valid for another.
func (s *Signer) mac(purpose, value string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(purpose))
	m.Write([]byte{0})
	m.Write([]byte(value))
	return m.Sum(nil)
}
