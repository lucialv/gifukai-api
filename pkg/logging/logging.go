package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

type Options struct {
	Service   string
	Env       string
	Version   string
	Level     slog.Level
	Pretty    bool
	LokiURL   string
	LokiUser  string
	LokiToken string
}

func New(opts Options) (*slog.Logger, io.Closer) {
	var stdout slog.Handler
	if opts.Pretty {
		stdout = newConsoleHandler(os.Stdout, opts.Level)
	} else {
		stdout = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: opts.Level})
	}

	var handler slog.Handler = stdout
	var closer io.Closer = noopCloser{}

	if opts.LokiURL != "" {
		loki := newLokiHandler(opts)
		handler = fanout{handlers: []slog.Handler{stdout, loki}}
		closer = loki
	}

	base := slog.New(handler).With(
		slog.String("service", opts.Service),
		slog.String("env", opts.Env),
	)
	if opts.Version != "" {
		base = base.With(slog.String("version", opts.Version))
	}
	return base, closer
}

func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// walkAttr resolves a and recurses into groups, calling emit for each leaf with
// its dotted key (group names joined by ".") :3
func walkAttr(prefix string, a slog.Attr, emit func(key string, v slog.Value)) {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			walkAttr(prefix+a.Key+".", ga, emit)
		}
		return
	}
	emit(prefix+a.Key, a.Value)
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// fanout sends every record to all wrapped handlers :3
type fanout struct{ handlers []slog.Handler }

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	for i, h := range f.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		// last handler can take the record directly, no clone needed :3
		if i == len(f.handlers)-1 {
			_ = h.Handle(ctx, r)
		} else {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return fanout{handlers: hs}
}

func (f fanout) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		hs[i] = h.WithGroup(name)
	}
	return fanout{handlers: hs}
}
