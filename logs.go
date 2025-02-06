package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/goccy/go-json"
)

//go:embed files/index.html
var indexhtml []byte

type counter map[string]int

func (o *Opt) createServiceLog() error {
	day := time.Now().Format("20060102")
	path := filepath.Join(o.Data, fmt.Sprintf("log%s.txt", day))

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func (o *Opt) appendServiceLog(log *ServiceLog) error {
	day := log.Time.Format("20060102")
	path := filepath.Join(o.Data, fmt.Sprintf("log%s.txt", day))

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(log)
}

func (o *Opt) loadLog(_ context.Context, d time.Time) (time.Time, counter, counter, counter, counter, error) {
	day := d.Format("20060102")
	path := filepath.Join(o.Data, fmt.Sprintf("log%s.txt", day))

	ok := map[string]int{}
	failed := map[string]int{}
	latestOk := map[string]int{}
	latestFailed := map[string]int{}

	file, err := os.Open(path)
	if err != nil {
		return time.Now(), ok, failed, latestOk, latestFailed, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lastUpdated := time.Now()
	// 各行を読み込み
	for scanner.Scan() {
		servicelog := &ServiceLog{}
		// JSON をデコード
		err := json.Unmarshal(scanner.Bytes(), servicelog)
		if err != nil {
			slog.Warn("Error decoding JSON", slog.Any("error", err))
			continue
		}
		lastUpdated = servicelog.Time
		k := joinName(servicelog.CategoryName, servicelog.Name)

		if servicelog.Status == 0 {
			if count, exist := ok[k]; exist {
				ok[k] = count + 1
			} else {
				ok[k] = 1
			}
		} else {
			if count, exist := failed[k]; exist {
				failed[k] = count + 1
			} else {
				failed[k] = 1
			}
		}

		now := time.Now()
		diff := now.Sub(servicelog.Time)
		if diff <= o.config.LatestTimeRange.Duration {
			if servicelog.Status == 0 {
				if count, exist := latestOk[k]; exist {
					latestOk[k] = count + 1
				} else {
					latestOk[k] = 1
				}
			} else {
				if count, exist := latestFailed[k]; exist {
					latestFailed[k] = count + 1
				} else {
					latestFailed[k] = 1
				}
			}
		}
	}

	// エラーチェック
	if err := scanner.Err(); err != nil {
		slog.Warn("Error reading file", slog.Any("error", err))
	}

	return lastUpdated, ok, failed, latestOk, latestFailed, nil
}

func (o *Opt) loadLogs(ctx context.Context) {
	d := time.Now()
	days := make([]string, 0, 10)
	days = append(days, o.config.LatestTimeRange.ShortString())

	// initilize
	for _, categeory := range o.config.Categories {
		for _, service := range categeory.Services {
			service.LatestStatus = NoDATA
			service.LatestStatusAt = time.Now()
			history := []*statusText{NoDATA, NoDATA, NoDATA, NoDATA, NoDATA, NoDATA, NoDATA}
			service.StatusHistory = history
		}
	}
	for i := 0; i < 7; i++ {
		days = append(days, d.Format("01/02"))
		lastUpdated, ok, failed, latestOk, latestFailed, err := o.loadLog(ctx, d)
		d = d.Add(-1 * time.Hour * 24)
		if err != nil && err == os.ErrNotExist {
			slog.Warn("failed to loadlog", slog.Any("error", err))
			continue
		}
		if i == 0 {
			// latestをいれる
			for _, categeory := range o.config.Categories {
				for _, service := range categeory.Services {
					k := joinName(categeory.Name, service.Name)
					okCount := latestOk[k]
					ngCount := latestFailed[k]
					service.LatestStatusAt = lastUpdated
					if okCount == 0 && ngCount == 0 {
						service.LatestStatus = NoDATA
					} else if ngCount > 0 {
						service.LatestStatus = Warning
					} else {
						service.LatestStatus = Operational
					}
				}
			}
		}
		// 直近
		for _, categeory := range o.config.Categories {
			for _, service := range categeory.Services {
				k := joinName(categeory.Name, service.Name)
				okCount := ok[k]
				ngCount := failed[k]
				if okCount == 0 && ngCount == 0 {
					service.StatusHistory[i] = NoDATA
				} else if ngCount > 0 {
					service.StatusHistory[i] = Warning
				} else {
					service.StatusHistory[i] = Operational
				}
			}
		}
	}
	o.config.Days = days
	o.config.LastUpdatedAt = time.Now()
}

func (o *Opt) renderStatusPage(ctx context.Context) error {
	o.loadLogs(ctx)
	r := template.Must(template.New("index").Parse(string(indexhtml)))
	w := &bytes.Buffer{}
	err := r.ExecuteTemplate(w, "index", o.config)
	if err != nil {
		return err
	}
	o.htmlBlob = w.Bytes()
	return nil
}
