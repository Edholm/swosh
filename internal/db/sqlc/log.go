package sqlc

// This file is written by hand; sqlc leaves it alone.

import "log/slog"

// LogValue leaves out the payee and message. Anyone can create links, so
// they stay out of the log.
func (s Swosh) LogValue() slog.Value {
	attrs := []slog.Attr{slog.String("id", s.ID), slog.Int64("amount", int64(s.Amount))}
	if s.ExpiresOn != nil {
		attrs = append(attrs, slog.Time("expires_on", *s.ExpiresOn))
	}
	return slog.GroupValue(attrs...)
}

// LogValue leaves out the password hash.
func (u User) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("username", u.Username),
		slog.Bool("enabled", u.Enabled),
		slog.Any("authorities", u.Authorities),
	)
}
