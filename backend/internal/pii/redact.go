package pii

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
)

// Error text from outside (SMTP replies, database details, M-Pesa answers)
// can quote an email address or phone number. RedactHandler masks both in
// every log message and attribute before it is written (Stage 17c,
// CLAUDE.md: never log full phone numbers).
var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// Kenyan mobiles in the forms people and systems write them:
	// +2547XXXXXXXX, 2547XXXXXXXX, 07XXXXXXXX, 01XXXXXXXX (also with 1 for 7).
	phoneRe = regexp.MustCompile(`(?:\+?254|\b0)[17]\d{8}\b`)
)

// Redact masks email addresses (keeping the domain) and phone numbers
// (keeping the last three digits) in s.
func Redact(s string) string {
	s = emailRe.ReplaceAllStringFunc(s, func(m string) string {
		for i := len(m) - 1; i >= 0; i-- {
			if m[i] == '@' {
				return "•••" + m[i:]
			}
		}
		return "•••"
	})
	return phoneRe.ReplaceAllStringFunc(s, func(m string) string { return "•••" + m[len(m)-3:] })
}

// RedactHandler wraps a slog.Handler and redacts every string it carries.
type RedactHandler struct{ next slog.Handler }

// NewRedactHandler wraps next.
func NewRedactHandler(next slog.Handler) *RedactHandler { return &RedactHandler{next: next} }

func (h *RedactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, Redact(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, out)
}

func (h *RedactHandler) WithAttrs(as []slog.Attr) slog.Handler {
	red := make([]slog.Attr, len(as))
	for i, a := range as {
		red[i] = redactAttr(a)
	}
	return &RedactHandler{next: h.next.WithAttrs(red)}
}

func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{next: h.next.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Redact(v.String()))
	case slog.KindGroup:
		g := v.Group()
		out := make([]any, len(g))
		for i, x := range g {
			out[i] = redactAttr(x)
		}
		return slog.Group(a.Key, out...)
	case slog.KindAny:
		switch x := v.Any().(type) {
		case error:
			return slog.String(a.Key, Redact(x.Error()))
		case fmt.Stringer:
			return slog.String(a.Key, Redact(x.String()))
		case string:
			return slog.String(a.Key, Redact(x))
		default:
			return slog.String(a.Key, Redact(fmt.Sprint(x)))
		}
	}
	return slog.Attr{Key: a.Key, Value: v}
}
