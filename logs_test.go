package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestOpt(t *testing.T) *Opt {
	tmpDir := t.TempDir()

	tomlContent := `
latest_time_range = "2h"
[[category]]
name = "Web"
comment = "Web services"

[[category.service]]
name = "Google"
command = ["ping", "google.com"]
`
	path := writeTempToml(t, tomlContent)
	conf, err := loadToml(path)
	if err != nil {
		t.Fatalf("loadToml failed: %v", err)
	}
	return &Opt{
		Data:   tmpDir,
		config: conf,
	}
}

func writeServiceLog(t *testing.T, dir string, logs []*ServiceLog, day string) {
	path := filepath.Join(dir, "log"+day+".txt")
	var buf bytes.Buffer
	for _, l := range logs {
		b, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndAppendServiceLog(t *testing.T) {
	opt := newTestOpt(t)
	err := opt.createServiceLog()
	if err != nil {
		t.Fatalf("createServiceLog failed: %v", err)
	}
	log := &ServiceLog{
		Time:         time.Now(),
		Name:         "Google",
		CategoryName: "Web",
		Command:      []string{"ping", "google.com"},
		Status:       0,
	}
	err = opt.appendServiceLog(log)
	if err != nil {
		t.Fatalf("appendServiceLog failed: %v", err)
	}
	// Check file exists and contains log
	day := log.Time.Format("20060102")
	path := filepath.Join(opt.Data, "log"+day+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("log file not found: %v", err)
	}
	if !strings.Contains(string(data), "Google") {
		t.Errorf("log file does not contain service name")
	}
}

func TestLoadServiceLog(t *testing.T) {
	opt := newTestOpt(t)
	now := time.Now()
	logs := []*ServiceLog{
		{Time: now.Add(-1 * time.Hour), Name: "Google", CategoryName: "Web", Command: []string{"ping", "google.com"}, Status: 0},
		{Time: now.Add(-30 * time.Minute), Name: "Google", CategoryName: "Web", Command: []string{"ping", "google.com"}, Status: 1},
	}
	day := now.Format("20060102")
	writeServiceLog(t, opt.Data, logs, day)
	_, all, latest, err := opt.loadServiceLog(context.Background(), now)
	if err != nil {
		t.Fatalf("loadServiceLog failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all logs = %d, want 2", len(all))
	}
	if len(latest) != 2 {
		t.Errorf("latest logs = %d, want 2", len(latest))
	}
}

func TestLoadServiceLog_FileNotFound(t *testing.T) {
	opt := newTestOpt(t)
	_, all, latest, err := opt.loadServiceLog(context.Background(), time.Now().AddDate(0, 0, -10))
	if err == nil {
		t.Fatal("expected error for missing log file")
	}
	if len(all) != 0 || len(latest) != 0 {
		t.Errorf("logs should be empty on file not found")
	}
}

func TestSameCommand(t *testing.T) {
	a := []string{"ping", "google.com"}
	b := []string{"ping", "google.com"}
	c := []string{"curl", "google.com"}
	if !sameCommand(a, b) {
		t.Error("sameCommand should return true for identical slices")
	}
	if sameCommand(a, c) {
		t.Error("sameCommand should return false for different slices")
	}
}

func TestCountByService(t *testing.T) {
	opt := newTestOpt(t)
	service := &Service{Name: "Google", categoryName: "Web", Command: []string{"ping", "google.com"}}
	logs := []*ServiceLog{
		{Name: "Google", CategoryName: "Web", Command: []string{"ping", "google.com"}, Status: 0},
		{Name: "Google", CategoryName: "Web", Command: []string{"ping", "google.com"}, Status: 1},
		{Name: "Other", CategoryName: "Web", Command: []string{"ping", "other.com"}, Status: 0},
	}
	ok, fail := opt.countByService(logs, service)
	if ok != 1 || fail != 1 {
		t.Errorf("countByService = %d ok, %d fail; want 1 ok, 1 fail", ok, fail)
	}
}

func TestLoadLogAndRenderStatusPage(t *testing.T) {
	opt := newTestOpt(t)
	now := time.Now()
	// Write logs for today and yesterday
	logsToday := []*ServiceLog{
		{Time: now.Add(-1 * time.Hour), Name: "Google", CategoryName: "Web", Command: []string{"ping", "google.com"}, Status: 0},
	}
	logsYesterday := []*ServiceLog{
		{Time: now.Add(-25 * time.Hour), Name: "Google", CategoryName: "Web", Command: []string{"ping", "google.com"}, Status: 1},
	}
	writeServiceLog(t, opt.Data, logsToday, now.Format("20060102"))
	writeServiceLog(t, opt.Data, logsYesterday, now.AddDate(0, 0, -1).Format("20060102"))

	err := opt.renderStatusPage(context.Background())
	if err != nil {
		t.Fatalf("renderStatusPage failed: %v", err)
	}
	if len(opt.htmlBlob) == 0 {
		t.Errorf("htmlBlob is empty after renderStatusPage")
	}
	// Check status updated
	svc := opt.config.Categories[0].Services[0]
	if svc.LatestStatus == nil {
		t.Errorf("LatestStatus not set")
	}
	if svc.StatusHistory == nil || len(svc.StatusHistory) != 7 {
		t.Errorf("StatusHistory not set or wrong length")
	}
}

func TestLoadLog_NoLogs(t *testing.T) {
	opt := newTestOpt(t)
	err := opt.renderStatusPage(context.Background())
	if err != nil {
		t.Fatalf("renderStatusPage failed: %v", err)
	}
	svc := opt.config.Categories[0].Services[0]
	if svc.LatestStatus != NoDATA {
		t.Errorf("LatestStatus = %v, want NoDATA", svc.LatestStatus)
	}
}
