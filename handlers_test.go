package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func parseJSONLogEntry(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("log is empty")
	}
	entry := map[string]any{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	return entry
}

func TestRequestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	e := echo.New()
	e.Logger = logger
	e.Use(RequestLogger(middleware.DefaultSkipper))
	e.GET("/hello", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/hello", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	entry := parseJSONLogEntry(t, &buf)
	if entry["msg"] != "REQUEST" {
		t.Fatalf("msg = %v, want REQUEST", entry["msg"])
	}
	if entry["method"] != "GET" {
		t.Errorf("method = %v, want GET", entry["method"])
	}
	uri, ok := entry["uri"].(string)
	if !ok || !strings.HasSuffix(uri, "/hello") {
		t.Errorf("uri = %v, want suffix /hello", entry["uri"])
	}
	status, ok := entry["status"].(float64)
	if !ok || int(status) != http.StatusOK {
		t.Errorf("status = %v, want %d", entry["status"], http.StatusOK)
	}
}

func TestRequestLogger_SkipperSkips(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	e := echo.New()
	e.Logger = logger
	e.Use(RequestLogger(func(c *echo.Context) bool { return true }))
	e.GET("/skip", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/skip", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	out := strings.TrimSpace(buf.String())
	if out != "" {
		t.Fatalf("log should be empty when skipped: %q", out)
	}
}
