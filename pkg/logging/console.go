package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// ANSI colors :3
const (
	cReset   = "\033[0m"
	cDim     = "\033[2m"
	cBold    = "\033[1m"
	cRed     = "\033[31m"
	cGreen   = "\033[32m"
	cYellow  = "\033[33m"
	cMagenta = "\033[35m"
	cCyan    = "\033[36m"
)

var consoleKeysHidden = map[string]bool{"service": true, "env": true, "version": true}

type consoleHandler struct {
	mu     *sync.Mutex
	w      io.Writer
	level  slog.Level
	color  bool
	attrs  []kv
	prefix string
}

type kv struct{ key, val string }

func newConsoleHandler(w io.Writer, level slog.Level) *consoleHandler {
	return &consoleHandler{mu: &sync.Mutex{}, w: w, level: level, color: isTerminal(w)}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	ts := r.Time.Format("15:04:05.000")
	b.WriteString(h.paint(cDim, ts))
	b.WriteByte(' ')
	b.WriteString(h.levelTag(r.Level))
	b.WriteByte(' ')
	b.WriteString(h.paint(cBold, r.Message))

	for _, a := range h.attrs {
		h.writeKV(&b, a.key, a.val)
	}
	r.Attrs(func(a slog.Attr) bool {
		h.appendAttr(&b, h.prefix, a)
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *consoleHandler) appendAttr(b *strings.Builder, prefix string, a slog.Attr) {
	walkAttr(prefix, a, func(key string, v slog.Value) {
		if !consoleKeysHidden[key] {
			h.writeKV(b, key, formatValue(v))
		}
	})
}

func (h *consoleHandler) writeKV(b *strings.Builder, key, val string) {
	b.WriteByte(' ')
	switch key {
	case "component":
		b.WriteString(h.paint(cMagenta, key+"="+val))
	case "event":
		b.WriteString(h.paint(cCyan, key+"="+val))
	case "error":
		b.WriteString(h.paint(cRed, key+"="+val))
	default:
		b.WriteString(h.paint(cDim, key+"=") + val)
	}
}

func (h *consoleHandler) levelTag(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return h.paint(cRed, "ERR")
	case l >= slog.LevelWarn:
		return h.paint(cYellow, "WRN")
	case l >= slog.LevelInfo:
		return h.paint(cGreen, "INF")
	default:
		return h.paint(cCyan, "DBG")
	}
}

func (h *consoleHandler) paint(color, s string) string {
	if !h.color {
		return s
	}
	return color + s + cReset
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := h.clone()
	for _, a := range attrs {
		walkAttr(nh.prefix, a, func(key string, v slog.Value) {
			if !consoleKeysHidden[key] {
				nh.attrs = append(nh.attrs, kv{key, formatValue(v)})
			}
		})
	}
	return nh
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	nh.prefix += name + "."
	return nh
}

func (h *consoleHandler) clone() *consoleHandler {
	attrs := make([]kv, len(h.attrs))
	copy(attrs, h.attrs)
	return &consoleHandler{mu: h.mu, w: h.w, level: h.level, color: h.color, attrs: attrs, prefix: h.prefix}
}

func formatValue(v slog.Value) string {
	if v.Kind() == slog.KindAny {
		if e, ok := v.Any().(error); ok {
			return e.Error()
		}
	}
	s := v.String()
	if strings.ContainsAny(s, " \t") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
