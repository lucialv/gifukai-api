package middlewares

import (
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/lucialv/gifukai-api/pkg/logging"
	u "github.com/lucialv/gifukai-api/pkg/utils"

	"github.com/go-chi/chi/v5/middleware"
)

var sensitiveQueryKeys = []string{"code", "state", "token", "access_token"}

func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := middleware.GetReqID(r.Context())
			ip := clientIP(r)

			reqLog := logger.With(
				slog.String("component", "http"),
				slog.String("request_id", reqID),
				slog.String("ip", ip),
			)
			ctx := logging.WithLogger(r.Context(), reqLog)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))

			attrs := []slog.Attr{
				slog.String("event", "http_request"),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("user_agent", r.UserAgent()),
			}
			if values := r.URL.Query(); len(values) > 0 {
				for _, k := range sensitiveQueryKeys {
					if values.Has(k) {
						values.Set(k, "REDACTED")
					}
				}
				attrs = append(attrs, slog.String("query", values.Encode()))
				if p := values.Get("pairing"); p != "" {
					attrs = append(attrs, slog.String("pairing", p))
				}
			}
			reqLog.LogAttrs(r.Context(), levelForStatus(ww.Status()), "request", attrs...)
		})
	}
}

func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
				logging.FromContext(r.Context()).Error("panic recovered",
					slog.String("event", "panic"),
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)
				u.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
