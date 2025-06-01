package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTempToml(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.toml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp toml: %v", err)
	}
	return tmpFile
}

func TestLoadToml_Basic(t *testing.T) {
	tomlContent := `
title = "Test Board"
favicon = "/favicon.ico"
worker_interval = "2m"
worker_timeout = "10s"
num_of_worker = 2
max_check_attempts = 5
retry_interval = "3s"
latest_time_range = "2h"

[[category]]
name = "Web"
comment = "Web services"
hide = false

  [[category.service]]
  name = "Google"
  command = ["ping", "google.com"]
`
	path := writeTempToml(t, tomlContent)
	conf, err := loadToml(path)
	if err != nil {
		t.Fatalf("loadToml failed: %v", err)
	}

	if conf.Title != "Test Board" {
		t.Errorf("Title = %q, want %q", conf.Title, "Test Board")
	}
	if conf.Favicon != "/favicon.ico" {
		t.Errorf("Favicon = %q, want %q", conf.Favicon, "/favicon.ico")
	}
	if conf.WorkerInterval.Duration != 2*time.Minute {
		t.Errorf("WorkerInterval = %v, want 2m", conf.WorkerInterval)
	}
	if conf.WorkerTimeout.Duration != 10*time.Second {
		t.Errorf("WorkerTimeout = %v, want 10s", conf.WorkerTimeout)
	}
	if conf.NumOfWorker != 2 {
		t.Errorf("NumOfWorker = %d, want 2", conf.NumOfWorker)
	}
	if conf.MaxCheckAttempts != 5 {
		t.Errorf("MaxCheckAttempts = %d, want 5", conf.MaxCheckAttempts)
	}
	if conf.RetryInterval.Duration != 3*time.Second {
		t.Errorf("RetryInterval = %v, want 3s", conf.RetryInterval)
	}
	if conf.LatestTimeRange.Duration != 2*time.Hour {
		t.Errorf("LatestTimeRange = %v, want 2h", conf.LatestTimeRange)
	}
	if len(conf.Categories) != 1 {
		t.Fatalf("Categories len = %d, want 1", len(conf.Categories))
	}
	cat := conf.Categories[0]
	if cat.Name != "Web" {
		t.Errorf("Category.Name = %q, want %q", cat.Name, "Web")
	}
	if cat.Comment != "Web services" {
		t.Errorf("Category.Comment = %q, want %q", cat.Comment, "Web services")
	}
	if cat.Hide != false {
		t.Errorf("Category.Hide = %v, want false", cat.Hide)
	}
	if len(cat.Services) != 1 {
		t.Fatalf("Services len = %d, want 1", len(cat.Services))
	}
	svc := cat.Services[0]
	if svc.Name != "Google" {
		t.Errorf("Service.Name = %q, want %q", svc.Name, "Google")
	}
	if svc.categoryName != "Web" {
		t.Errorf("Service.categoryName = %q, want %q", svc.categoryName, "Web")
	}
	if len(svc.Command) != 2 || svc.Command[0] != "ping" || svc.Command[1] != "google.com" {
		t.Errorf("Service.Command = %v, want [ping google.com]", svc.Command)
	}
	if conf.PoweredBy == nil || !strings.Contains(conf.PoweredBy.html, "Powered by statusboard") {
		t.Errorf("PoweredBy = %v, want contains 'Powered by statusboard'", conf.PoweredBy)
	}
	if conf.LastUpdatedAt.IsZero() {
		t.Errorf("LastUpdatedAt should be set")
	}
}

func TestLoadToml_Defaults(t *testing.T) {
	tomlContent := `
title = "Defaults Test"
[[category]]
name = "Cat"
comment = "C"
  [[category.service]]
  name = "Svc"
  command = ["echo"]
`
	path := writeTempToml(t, tomlContent)
	conf, err := loadToml(path)
	if err != nil {
		t.Fatalf("loadToml failed: %v", err)
	}
	if conf.NumOfWorker != 4 {
		t.Errorf("NumOfWorker = %d, want 4", conf.NumOfWorker)
	}
	if conf.WorkerInterval.Duration != 5*time.Minute {
		t.Errorf("WorkerInterval = %v, want 5m", conf.WorkerInterval)
	}
	if conf.WorkerTimeout.Duration != 30*time.Second {
		t.Errorf("WorkerTimeout = %v, want 30s", conf.WorkerTimeout)
	}
	if conf.LatestTimeRange.Duration != 1*time.Hour {
		t.Errorf("LatestTimeRange = %v, want 1h", conf.LatestTimeRange)
	}
	if conf.MaxCheckAttempts != 3 {
		t.Errorf("MaxCheckAttempts = %d, want 3", conf.MaxCheckAttempts)
	}
	if conf.RetryInterval.Duration != 5*time.Second {
		t.Errorf("RetryInterval = %v, want 5s", conf.RetryInterval)
	}
	if conf.PoweredBy == nil || !strings.Contains(conf.PoweredBy.html, "Powered by statusboard") {
		t.Errorf("PoweredBy = %v, want contains 'Powered by statusboard'", conf.PoweredBy)
	}
}

func TestLoadToml_FileNotFound(t *testing.T) {
	_, err := loadToml("not_exist.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadToml_InvalidToml(t *testing.T) {
	tomlContent := `invalid =`
	path := writeTempToml(t, tomlContent)
	_, err := loadToml(path)
	if err == nil {
		t.Fatal("expected error for invalid toml, got nil")
	}
}
