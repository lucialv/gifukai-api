package logging

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var labelKeys = map[string]bool{"service": true, "env": true, "component": true}

type lokiHandler struct {
	client *lokiClient
	level  slog.Level
	labels map[string]string
	fields map[string]any
	prefix string
}

func newLokiHandler(opts Options) *lokiHandler {
	c := &lokiClient{
		url:   normalizeLokiURL(opts.LokiURL),
		user:  opts.LokiUser,
		token: opts.LokiToken,
		http:  &http.Client{Timeout: 10 * time.Second},
		ch:    make(chan lokiEntry, 4096),
		done:  make(chan struct{}),
	}
	c.wg.Add(1)
	go c.run()
	return &lokiHandler{
		client: c,
		level:  opts.Level,
		labels: map[string]string{},
		fields: map[string]any{},
	}
}

func normalizeLokiURL(raw string) string {
	const pushPath = "/loki/api/v1/push"
	url := strings.TrimRight(strings.TrimSpace(raw), "/")
	if url == "" || strings.HasSuffix(url, pushPath) {
		return url
	}
	return url + pushPath
}

func (h *lokiHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *lokiHandler) Handle(_ context.Context, r slog.Record) error {
	labels := map[string]string{"level": levelString(r.Level)}
	maps.Copy(labels, h.labels)
	fields := make(map[string]any, len(h.fields)+r.NumAttrs()+1)
	maps.Copy(fields, h.fields)
	fields["msg"] = r.Message
	r.Attrs(func(a slog.Attr) bool {
		applyAttr(h.prefix, a, labels, fields)
		return true
	})

	select {
	case h.client.ch <- lokiEntry{ts: r.Time, labels: labels, fields: fields}:
	default:
		h.client.dropped.Add(1)
	}
	return nil
}

func (h *lokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := h.clone()
	for _, a := range attrs {
		applyAttr(nh.prefix, a, nh.labels, nh.fields)
	}
	return nh
}

func (h *lokiHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	nh.prefix += name + "."
	return nh
}

func (h *lokiHandler) clone() *lokiHandler {
	labels := make(map[string]string, len(h.labels))
	maps.Copy(labels, h.labels)
	fields := make(map[string]any, len(h.fields))
	maps.Copy(fields, h.fields)
	return &lokiHandler{client: h.client, level: h.level, labels: labels, fields: fields, prefix: h.prefix}
}

func (h *lokiHandler) Close() error { return h.client.Close() }

func applyAttr(prefix string, a slog.Attr, labels map[string]string, fields map[string]any) {
	walkAttr(prefix, a, func(key string, v slog.Value) {
		if labelKeys[key] {
			labels[key] = v.String()
			return
		}
		fields[key] = attrValue(v)
	})
}

func attrValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindAny:
		if e, ok := v.Any().(error); ok {
			return e.Error()
		}
		return v.Any()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	default:
		return v.Any()
	}
}

func levelString(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

type lokiEntry struct {
	ts     time.Time
	labels map[string]string
	fields map[string]any
}

type lokiClient struct {
	url, user, token string
	http             *http.Client
	ch               chan lokiEntry
	done             chan struct{}
	wg               sync.WaitGroup
	dropped          atomic.Int64
	closeOnce        sync.Once
}

func (c *lokiClient) run() {
	defer c.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	batch := make([]lokiEntry, 0, 256)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		c.send(batch)
		batch = batch[:0]
	}
	for {
		select {
		case e := <-c.ch:
			batch = append(batch, e)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-c.done:
			for {
				select {
				case e := <-c.ch:
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (c *lokiClient) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	c.wg.Wait()
	return nil
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

func (c *lokiClient) send(batch []lokiEntry) {
	byStream := map[string]*lokiStream{}
	for _, e := range batch {
		line, err := json.Marshal(e.fields)
		if err != nil {
			continue
		}
		key := streamKey(e.labels)
		s, ok := byStream[key]
		if !ok {
			s = &lokiStream{Stream: e.labels}
			byStream[key] = s
		}
		s.Values = append(s.Values, [2]string{strconv.FormatInt(e.ts.UnixNano(), 10), string(line)})
	}

	push := lokiPush{Streams: make([]lokiStream, 0, len(byStream))}
	for _, s := range byStream {
		sort.Slice(s.Values, func(i, j int) bool { return s.Values[i][0] < s.Values[j][0] })
		push.Streams = append(push.Streams, *s)
	}

	body, err := json.Marshal(push)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(body)
	_ = gz.Close()

	req, err := http.NewRequest(http.MethodPost, c.url, &buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loki: build request failed: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if c.user != "" {
		req.SetBasicAuth(c.user, c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loki: push failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		fmt.Fprintf(os.Stderr, "loki: push status %d: %s\n", resp.StatusCode, b)
	}
}

func streamKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}
	return b.String()
}
