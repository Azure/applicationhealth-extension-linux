package logging

import (
	"context"
	"io"
	"log/slog"
)

type ExtensionSlogHandler struct {
	slog.Handler
}

func NewExtensionSlogHandler(w io.Writer, opts *slog.HandlerOptions) *ExtensionSlogHandler {
	// ignore opts for now
	return &ExtensionSlogHandler{
		Handler: slog.NewTextHandler(w, &slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.MessageKey && a.Value.String() == "" {
					return slog.Attr{}
				}
				return a
			},
		}),
	}
}

func (h *ExtensionSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	msg := record.Message
	if msg != "" {
		record.Message = ""
		record.AddAttrs(slog.Attr{Key: "event", Value: slog.StringValue(msg)})
	}
	return h.Handler.Handle(ctx, record)
}

func (h *ExtensionSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ExtensionSlogHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ExtensionSlogHandler) WithGroup(name string) slog.Handler {
	return &ExtensionSlogHandler{Handler: h.Handler.WithGroup(name)}
}
