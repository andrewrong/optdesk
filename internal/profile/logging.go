package profile

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
)

// rawID is the shape of a broker account id.
var rawID = regexp.MustCompile(`\b[DU]U?\d{6,8}\b`)

const redacted = "[redacted]"

// NewLogHandler implements NewLogHandlerFunc: it drops every attribute whose key is not
// in the profile's whitelist, and replaces a whitelisted string value of the shape of
// an account id by "[redacted]".
func NewLogHandler(p Profile, next slog.Handler) slog.Handler {
	return &handler{allow: slices.Clone(p.LogFields), next: next}
}

type handler struct {
	allow []string
	next  slog.Handler
}

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool { return h.next.Enabled(ctx, l) }

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a, ok := h.clean(a); ok {
			out.AddAttrs(a)
		}
		return true
	})
	return h.next.Handle(ctx, out)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var kept []slog.Attr
	for _, a := range attrs {
		if a, ok := h.clean(a); ok {
			kept = append(kept, a)
		}
	}
	return &handler{allow: h.allow, next: h.next.WithAttrs(kept)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{allow: h.allow, next: h.next.WithGroup(name)}
}

// clean returns the attribute as it may pass, or false when it is dropped.
func (h *handler) clean(a slog.Attr) (slog.Attr, bool) {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		var kept []slog.Attr
		for _, g := range a.Value.Group() {
			if g, ok := h.clean(g); ok {
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 {
			return a, false
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(kept...)}, true
	}
	if !slices.Contains(h.allow, a.Key) {
		return a, false
	}
	if a.Value.Kind() == slog.KindString && rawID.MatchString(a.Value.String()) {
		a.Value = slog.StringValue(redacted)
	}
	return a, true
}
