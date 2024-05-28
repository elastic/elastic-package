// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type Handler struct {
	handler     slog.Handler
	mutex       *sync.Mutex
	out         io.Writer
	bytes       *bytes.Buffer
	replaceAttr func(groups []string, a slog.Attr) slog.Attr
}

func newHandler(out io.Writer, opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	buffer := &bytes.Buffer{}
	originalReplaceAttr := opts.ReplaceAttr
	opts.ReplaceAttr = suppressDefaults(opts.ReplaceAttr)
	return &Handler{
		out:         out,
		handler:     slog.NewJSONHandler(buffer, opts),
		mutex:       &sync.Mutex{},
		bytes:       buffer,
		replaceAttr: originalReplaceAttr,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{handler: h.handler.WithAttrs(attrs), out: h.out, mutex: h.mutex, bytes: h.bytes, replaceAttr: h.replaceAttr}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{handler: h.handler.WithGroup(name), out: h.out, mutex: h.mutex, bytes: h.bytes, replaceAttr: h.replaceAttr}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	var level, timestamp, msg string
	levelAttr := slog.Attr{
		Key:   slog.LevelKey,
		Value: slog.AnyValue(r.Level),
	}

	timeAttr := slog.Attr{
		Key:   slog.TimeKey,
		Value: slog.TimeValue(r.Time),
	}

	msgAttr := slog.Attr{
		Key:   slog.MessageKey,
		Value: slog.StringValue(r.Message),
	}

	if h.replaceAttr != nil {
		levelAttr = h.replaceAttr([]string{}, levelAttr)
		timeAttr = h.replaceAttr([]string{}, timeAttr)
		msgAttr = h.replaceAttr([]string{}, msgAttr)
	}

	if !levelAttr.Equal(slog.Attr{}) {
		level = fmt.Sprintf("%s:", levelAttr.Value.String())
	}
	if !timeAttr.Equal(slog.Attr{}) {
		timestamp = timeAttr.Value.String()
	}
	if !msgAttr.Equal(slog.Attr{}) {
		msg = msgAttr.Value.String()
	}

	attrs, err := h.computeAttrs(ctx, r)
	if err != nil {
		return err
	}
	// bytes, err := json.MarshalIndent(attrs, "", "  ")
	bytes, err := json.Marshal(attrs)
	if err != nil {
		return fmt.Errorf("error when marshaling attrs: %w", err)
	}

	builder := strings.Builder{}
	if len(timestamp) > 0 {
		builder.WriteString(timestamp)
		builder.WriteString(" ")
	}
	if len(level) > 0 {
		builder.WriteString(level)
		builder.WriteString(" ")
	}
	if len(msg) > 0 {
		builder.WriteString(msg)
		builder.WriteString(" ")
	}
	if len(bytes) > 0 && string(bytes) != "{}" {
		builder.WriteString(string(bytes))
	}

	_, err = io.WriteString(h.out, builder.String()+"\n")
	if err != nil {
		return err
	}

	return nil
}

func suppressDefaults(next func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey ||
			a.Key == slog.LevelKey ||
			a.Key == slog.MessageKey {
			return slog.Attr{}
		}
		if next == nil {
			return a
		}
		return next(groups, a)
	}
}

func (h *Handler) computeAttrs(ctx context.Context, r slog.Record) (map[string]any, error) {
	h.mutex.Lock()
	defer func() {
		h.bytes.Reset()
		h.mutex.Unlock()
	}()
	if err := h.handler.Handle(ctx, r); err != nil {
		return nil, fmt.Errorf("error when calling inner handler's Handle: %w", err)
	}

	var attrs map[string]any
	err := json.Unmarshal(h.bytes.Bytes(), &attrs)
	if err != nil {
		return nil, fmt.Errorf("error when unmarshaling inner handler's Handle result: %w", err)
	}
	return attrs, nil
}
