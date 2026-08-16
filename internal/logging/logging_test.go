package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelInfo,
		"bogus":   slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseFormat(t *testing.T) {
	cases := map[string]Format{
		"json":  FormatJSON,
		"JSON":  FormatJSON,
		"text":  FormatText,
		"":      FormatText,
		"bogus": FormatText,
	}
	for in, want := range cases {
		if got := ParseFormat(in); got != want {
			t.Errorf("ParseFormat(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-123")
	if got := TraceID(ctx); got != "trace-123" {
		t.Fatalf("TraceID() = %q, want %q", got, "trace-123")
	}
	if FromContext(ctx) == nil {
		t.Fatal("FromContext() returned nil")
	}

	// 未注入 traceID 时应回退到默认 logger，且 TraceID 为空串
	if got := TraceID(context.Background()); got != "" {
		t.Fatalf("TraceID(background) = %q, want empty", got)
	}
	if FromContext(context.Background()) == nil {
		t.Fatal("FromContext(background) returned nil")
	}
}
